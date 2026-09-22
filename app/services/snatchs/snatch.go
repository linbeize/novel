// Copyright 2017 Vckai Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snatchs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/astaxie/beego"
	"github.com/axgle/mahonia"
	"github.com/vckai/novel/app/utils"
	"github.com/vckai/novel/app/utils/log"

	xhttp "github.com/vckai/novel/app/librarys/net/http"
	"github.com/vckai/novel/app/models"
)

const (
	// 默认分类
	DEF_CATE_ID = 13

	// 请求失败重试次数
	RETRY = 5

	// 采集请求的最小间隔（默认值，可由后台配置覆盖）
	DEFAULT_SNATCH_INTERVAL = 1500 * time.Millisecond

	// 随机抖动上限（默认值），避免固定节奏被识别为爬虫
	DEFAULT_SNATCH_JITTER = 800 * time.Millisecond

	// 被限流时的默认冷却时长（对方未返回 Retry-After 时使用）
	DEFAULT_RATE_LIMIT_COOLDOWN = 60 * time.Second
)

// 采集节奏提供者：由 services 包注入，读取后台可配置的间隔值。
// 使用函数注入是为了避免 snatchs 反向依赖 services（循环引用）。
var paceProvider func() (interval, jitter time.Duration)

// 由外部注入采集节奏来源
func SetPaceProvider(fn func() (time.Duration, time.Duration)) {
	paceProvider = fn
}

// 获取当前采集节奏
func snatchPace() (interval, jitter time.Duration) {
	if paceProvider != nil {
		interval, jitter = paceProvider()
	}

	if interval <= 0 {
		interval = DEFAULT_SNATCH_INTERVAL
	}
	if jitter < 0 {
		jitter = 0
	}

	return interval, jitter
}

// 是否为「被限流」类响应
func isRateLimited(status int) bool {
	return status == http.StatusTooManyRequests || // 429
		status == http.StatusServiceUnavailable || // 503
		status == http.StatusForbidden || // 403 常被用于反爬拦截
		status == http.StatusBadGateway // 502（实测 5566xs 限流即返回此码）
}

// 计算限流后的冷却时长：优先尊重 Retry-After 头
func rateLimitCooldown(resp *http.Response) time.Duration {
	if resp != nil {
		// Retry-After: 秒数
		if v := resp.Header.Get("Retry-After"); v != "" {
			if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs > 0 {
				d := time.Duration(secs) * time.Second
				if d > 5*time.Minute {
					d = 5 * time.Minute
				}
				return d
			}
		}
	}

	return DEFAULT_RATE_LIMIT_COOLDOWN
}

var (
	ErrNotProvider  = errors.New("没有获取到采集点")
	ErrNotResp      = errors.New("没有返回")
	ErrNotRule      = errors.New("没有采集规则")
	ErrNotNov       = errors.New("获取小说失败")
	ErrNotURL       = errors.New("没有传入URL地址")
	ErrNotNovName   = errors.New("获取小说书名失败")
	ErrNotNovURL    = errors.New("获取小说URL失败")
	ErrNotNovAuthor = errors.New("获取小说作者失败")
	ErrNotChapTitle = errors.New("获取小说章节标题失败")
	ErrNotChapDesc  = errors.New("获取小说章节内容失败")
	ErrNotFindURL   = errors.New("没有配置搜索页URL地址")
	ErrInvalidURL   = errors.New("无效的URL地址")

	// 章节正文分页后缀，例如：
	//	122701_2.html  -> 122701.html   （5566xs 用下划线）
	//	41329-2.html   -> 41329.html    （shudugu 用连字符）
	//	41329_2.html / 41329-2.html / 41329.2.html 均需归一化
	//
	// 注意：必须同时兼容「下划线」与「连字符」两种分隔符。
	// 若只匹配下划线，遇到连字符站点（如速读谷）会判定为「非同一章」，
	// 从而在第二页前提前停止，导致每章只保存第一页。
	reChapterPageSuffix = regexp.MustCompile(`[_-]\d+(\.html?)$`)

	defaultFilterRules = []string{
		"\\<script[\\S\\s]+?\\</script\\>",
		"\\<style[\\S\\s]+?\\</style\\>",
	}
)

// 采集内容信息
type SnatchInfo struct {
	UseTime time.Duration
	Title   string
	Nov     *models.Novel
	Chap    *models.Chapter
	Url     string

	// Source 采集规则代号（如 bqgnovels）
	Source string

	// SiteName 采集站中文名，用于界面展示「来自哪个站」。
	// 原模板取的是 .Title（该字段实际为空），站点名显示不出来，故单独补充。
	SiteName string

	ChapterUrl string
	NextUrl    string
	PreUrl     string
}

type Snatch struct {
	proxyFunc func() string
}

func NewSnatch(proxyFunc func() string) *Snatch {
	return &Snatch{
		proxyFunc: proxyFunc,
	}
}

// 是否小说简介页面
func (this *Snatch) IsBookURL(provider *models.SnatchRule, rawurl string) bool {
	if provider == nil {
		return false
	}

	rule := provider.Rules

	if rule == nil {
		return false
	}

	rawurl = strings.TrimSpace(rawurl)
	re, _ := regexp.Compile("(?U)" + rule.IsBookURL)
	s := re.FindString(rawurl)

	if len(s) == 0 {
		return false
	}

	return true
}

// 是否小说是否爬虫页面
func (this *Snatch) IsCrawlerURL(provider *models.SnatchRule, rawurl string) bool {
	if provider == nil {
		return false
	}

	rule := provider.Rules

	if rule == nil {
		return false
	}

	if len(rule.IsCrawlerURL) == 0 {
		return true
	}

	rawurl = strings.TrimSpace(rawurl)

	re, _ := regexp.Compile("(?U)" + rule.IsCrawlerURL)
	s := re.FindString(rawurl)

	if len(s) == 0 {
		return false
	}

	return true
}

// 查找小说
func (this *Snatch) FindNovel(provider *models.SnatchRule, kw string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	// 部分站点改版后原搜索地址失效（但采集链路仍可用），
	// 此时改走其 JSON 接口搜索。见 search_api.go 的说明。
	if isDeadBqgnovelsSearch(provider) {
		list, err := this.searchBqgnovelsViaAPI(provider, kw, 1)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, ErrNotNovURL
		}
		return list[0], nil
	}

	t1 := time.Now()

	rule := provider.Rules

	if rule == nil {
		return nil, ErrNotRule
	}

	charset := provider.Charset
	if len(rule.FindCharset) > 0 {
		charset = rule.FindCharset
	}

	// 转换为gbk编码
	if charset == "GB18030" {
		enc := mahonia.NewEncoder(charset)
		kw = enc.ConvertString(kw)
	}

	if len(rule.FindURL) == 0 {
		return nil, ErrNotFindURL
	}

	kw = strings.TrimSpace(kw)

	// POST 搜索：部分站点的搜索表单只接受 POST，且需要额外参数
	// （如 biquge365 需 type=articlename）。规则里用如下写法表达：
	//
	//	POST:https://www.example.com/s.php|type=articlename&s={{kw}}
	//
	// 不带 POST: 前缀的仍走原有的 GET 拼接逻辑，不影响既有站点。
	if strings.HasPrefix(rule.FindURL, "POST:") {
		return this.findNovelByPost(provider, rule, charset, kw)
	}

	kw = url.QueryEscape(kw)

	// 解析URL
	u, err := url.Parse(rule.FindURL + kw)
	if err != nil {
		return nil, err
	}

	// 请求搜索页面
	doc, resp, err := this.newHtml(u.String(), charset, false)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, ErrNotResp
	}

	// 查找直接定向到小说简介页面
	novURL := resp.Header.Get("Location")
	if len(novURL) == 0 || !this.IsBookURL(provider, novURL) {
		// 获取小说介绍页面URL
		novURL, _ = doc.Find(rule.FindBookURLSelector).Attr("href")
	}

	if len(novURL) == 0 {
		return nil, ErrNotNovURL
	}

	novURL, err = this.genrateURL(u, novURL)
	if err != nil {
		return nil, err
	}

	if !this.IsBookURL(provider, novURL) {
		return nil, ErrInvalidURL
	}

	// 获取小说简介
	info, err := this.GetNovel(provider, novURL)
	if err != nil {
		return nil, err
	}

	info.UseTime = time.Since(t1)

	log.Debug(fmt.Sprintf("[%s]查找小说[%s]，使用时间：%v", provider.Name, info.Nov.Name, info.UseTime))

	return info, nil
}

// 获取一本小说
func (this *Snatch) GetNovel(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	rule := provider.Rules

	if rule == nil {
		return nil, ErrNotRule
	}

	t1 := time.Now()

	// 解析URL
	rawurl = strings.TrimSpace(rawurl)
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	// 请求采集
	doc, _, err := this.newHtml(rawurl, provider.Charset, true)
	if err != nil {
		return nil, err
	}

	nov := models.NewNovel()

	// 获取封面图片
	if len(rule.BookCoverAttr) == 0 {
		rule.BookCoverAttr = "src"
	}

	if img, ok := doc.Find(rule.BookCoverSelector).Attr(rule.BookCoverAttr); ok {
		nov.Cover = img

		if !strings.Contains(nov.Cover, "https://") && !strings.Contains(nov.Cover, "http://") && len(nov.Cover) > 0 {
			nov.Cover, _ = this.genrateURL(u, nov.Cover)
		}
	}

	// 默认封面图片直接重置空
	if len(rule.BookNoCover) > 0 && strings.Contains(nov.Cover, rule.BookNoCover) {
		nov.Cover = ""
	}

	// 获取小说类别
	if len(rule.BookCateAttr) > 0 {
		nov.CateName = doc.Find(rule.BookCateSelector).AttrOr(rule.BookCateAttr, "")
	} else {
		nov.CateName = doc.Find(rule.BookCateSelector).Text()
	}
	nov.CateName = this.filter(rule.BookCateFilter, nov.CateName)
	nov.CateName = strings.TrimSpace(nov.CateName)

	// 获取小说类别ID
	nov.CateId = DEF_CATE_ID
	for _, v := range provider.CateMaps {
		if v.Name == nov.CateName {
			nov.CateId = v.Id
			break
		}
	}

	// 获取小说标题
	if len(rule.BookTitleAttr) > 0 {
		nov.Name = doc.Find(rule.BookTitleSelector).AttrOr(rule.BookTitleAttr, "")
	} else {
		nov.Name = doc.Find(rule.BookTitleSelector).Text()
	}
	nov.Name = this.filter(rule.BookTitleFilter, nov.Name)
	nov.Name = strings.TrimSpace(nov.Name)
	if len(nov.Name) == 0 {
		return nil, ErrNotNovName
	}

	// 获取小说最新章节
	if len(rule.BookLastChapterTitleSelector) > 0 {
		if len(rule.BookLastChapterTitleAttr) > 0 {
			nov.ChapterTitle = doc.Find(rule.BookLastChapterTitleSelector).AttrOr(rule.BookLastChapterTitleAttr, "")
		} else {
			nov.ChapterTitle = doc.Find(rule.BookLastChapterTitleSelector).Text()
		}
		nov.ChapterTitle = strings.TrimSpace(nov.ChapterTitle)
	}

	// 获取小说作者
	if len(rule.BookAuthorAttr) > 0 {
		nov.Author = doc.Find(rule.BookAuthorSelector).AttrOr(rule.BookAuthorAttr, "")
	} else {
		nov.Author = doc.Find(rule.BookAuthorSelector).Text()
	}
	nov.Author = this.filter(rule.BookAuthorFilter, nov.Author)
	nov.Author = strings.TrimSpace(nov.Author)
	if len(nov.Author) == 0 {
		return nil, ErrNotNovAuthor
	}

	// 获取小说简介
	if len(rule.BookDescAttr) > 0 {
		nov.Desc = doc.Find(rule.BookDescSelector).AttrOr(rule.BookDescAttr, "")
	} else {
		nov.Desc, _ = doc.Find(rule.BookDescSelector).Html()
	}
	nov.Desc = this.filter(rule.BookDescFilter, nov.Desc)

	// 获取章节链接地址
	chapterLink := rawurl
	if len(rule.BookChapterURLSelector) > 0 {
		if len(rule.BookChapterURLAttr) == 0 {
			rule.BookChapterURLAttr = "href"
		}

		chapterLink = doc.Find(rule.BookChapterURLSelector).AttrOr(rule.BookChapterURLAttr, "")
		if len(chapterLink) == 0 {
			return nil, ErrNotNovURL
		}
		// 生成完整链接地址
		chapterLink, _ = this.genrateURL(u, chapterLink)
	}

	useTime := time.Since(t1)
	log.Debug(fmt.Sprintf("[%s]获取小说[%s]，使用时间：%v", provider.Name, nov.Name, useTime))

	return &SnatchInfo{
		ChapterUrl: chapterLink,
		Title:      provider.Name,
		Source:     provider.Code,
		Url:        rawurl,
		UseTime:    useTime,
		Nov:        nov,
	}, nil
}

// 获取小说章节内容
func (this *Snatch) GetChapter(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	rule := provider.Rules

	if rule == nil {
		return nil, ErrNotRule
	}

	t1 := time.Now()

	// 解析URL
	rawurl = strings.TrimSpace(rawurl)
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	// 请求小说详情页面
	doc, _, err := this.newHtml(rawurl, provider.Charset, true)
	if err != nil {
		return nil, err
	}

	chap := models.NewChapter()
	chap.Link = rawurl

	// 获取章节标题
	chap.Title = doc.Find(rule.InfoTitleSelector).Text()
	chap.Title = beego.HTML2str(chap.Title)
	chap.Title = this.filter(rule.InfoTitleFilter, chap.Title)
	chap.Title = strings.TrimSpace(chap.Title)
	if len(chap.Title) == 0 {
		return nil, ErrNotChapTitle
	}

	// 获取章节内容
	chap.Desc, _ = doc.Find(rule.InfoDescSelector).Html()
	chap.Desc = this.filter(rule.InfoDescFilter, chap.Desc)

	// 获取上一页
	preURL := ""
	if purl := doc.Find(rule.InfoPrePageSelector).AttrOr("href", ""); len(purl) > 0 {
		purl, _ = this.genrateURL(u, purl)
		if !this.IsBookURL(provider, purl) && rawurl != purl {
			preURL = purl
		}
	}

	// 获取下一页
	nextURL := ""
	if nurl := doc.Find(rule.InfoNextPageSelector).AttrOr("href", ""); len(nurl) > 0 {
		nurl, _ = this.genrateURL(u, nurl)
		if !this.IsBookURL(provider, nurl) && rawurl != nurl {
			nextURL = nurl
		}
	}

	log.Debug(fmt.Sprintf("[%s]获取小说章节内容[%s][%s]，使用时间：%v", provider.Name, chap.Title, rawurl, time.Since(t1)))

	return &SnatchInfo{
		Source:  provider.Code,
		Chap:    chap,
		UseTime: time.Since(t1),
		NextUrl: nextURL,
		PreUrl:  preURL,
	}, nil
}

// 章节正文分页相关
const (
	// 单章正文最多拼接的页数，防止异常规则造成死循环
	MAX_CHAPTER_PAGES = 10

	// 章节目录最多翻页数上限
	MAX_CATALOG_PAGES = 500

	// 分页正文之间的请求间隔，降低被目标站限流的概率
	CHAPTER_PAGE_INTERVAL = 300 * time.Millisecond
)

// 章节正文中的分页标记，如「第(1/3)页」，属于站点导航信息，需剔除
var reChapterPageMark = regexp.MustCompile(`第\s*\(\s*\d+\s*/\s*\d+\s*\)\s*页`)

// 去除 URL 中的分页后缀，得到「同一章」的基准地址
// 例如：122701.html / 122701_2.html / 122701_3.html 都归一化为 122701.html
func chapterBaseURL(rawurl string) string {
	return reChapterPageSuffix.ReplaceAllString(rawurl, "$1")
}

// 判断 next 是否仍是同一章的下一页
// 站点惯例：章内分页为 xxx_2.html，而末页的「下一章」指向 122702.html，
// 两者基准地址不同，据此终止拼接。
func sameChapterURL(cur, next string) bool {
	return chapterBaseURL(cur) == chapterBaseURL(next)
}

// 获取章节内容（自动拼接同章分页正文）
// 背景：部分站点（如 5566xs）把超长章节拆成多页
// （xxx.html、xxx_2.html、xxx_3.html），原实现只取第一页，
// 会丢失约 2/3 正文。此方法沿「下一页」链接持续拼接，
// 直到链接指向下一章或达到页数上限。
func (this *Snatch) GetChapterFull(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	// 第一页（含标题）
	info, err := this.GetChapter(provider, rawurl)
	if err != nil {
		return nil, err
	}

	rule := provider.Rules
	if rule == nil || len(rule.InfoNextPageSelector) == 0 {
		return info, nil
	}

	// 剔除首页正文里的分页标记
	info.Chap.Desc = reChapterPageMark.ReplaceAllString(info.Chap.Desc, "")

	cur := rawurl
	next := info.NextUrl
	pages := 1

	for pages < MAX_CHAPTER_PAGES && len(next) > 0 {
		// 下一页已不是同一章 -> 本章分页结束
		if !sameChapterURL(cur, next) {
			break
		}

		pg, err := this.GetChapterPage(provider, next, info.Chap.Title)
		if err != nil {
			log.Warn("章节分页获取失败：", next, err.Error())
			break
		}

		part := reChapterPageMark.ReplaceAllString(pg.Chap.Desc, "")
		part = this.filter(rule.InfoDescFilter, part)

		if len(strings.TrimSpace(part)) == 0 {
			break
		}

		info.Chap.Desc += part
		pages++

		cur = next
		next = pg.NextUrl

		time.Sleep(CHAPTER_PAGE_INTERVAL)
	}

	if pages > 1 {
		log.Debug(fmt.Sprintf("[%s]章节分页拼接完成[%s]，共 %d 页", provider.Name, info.Chap.Title, pages))
	}

	return info, nil
}

// 获取同章的分页正文（不校验标题，避免分页页标题为空导致失败）
func (this *Snatch) GetChapterPage(provider *models.SnatchRule, rawurl, title string) (*SnatchInfo, error) {
	rule := provider.Rules
	if rule == nil {
		return nil, ErrNotRule
	}

	doc, _, err := this.newHtml(rawurl, provider.Charset, true)
	if err != nil {
		return nil, err
	}

	chap := models.NewChapter()
	chap.Link = rawurl
	chap.Title = title

	chap.Desc, _ = doc.Find(rule.InfoDescSelector).Html()
	if len(chap.Desc) == 0 {
		return nil, ErrNotChapDesc
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	nextURL := ""
	if nurl := doc.Find(rule.InfoNextPageSelector).AttrOr("href", ""); len(nurl) > 0 {
		nurl, _ = this.genrateURL(u, nurl)
		if !this.IsBookURL(provider, nurl) && rawurl != nurl {
			nextURL = nurl
		}
	}

	return &SnatchInfo{
		Source:  provider.Code,
		Chap:    chap,
		NextUrl: nextURL,
	}, nil
}

// 由书籍 URL 推导其目录路径，用于 {book} 占位符
// 例如 https://www.5566xs.com/0/106/ -> /0/106/
func bookDirPath(rawurl string) string {
	u, err := url.Parse(strings.TrimSpace(rawurl))
	if err != nil {
		return ""
	}

	p := u.Path
	if !strings.HasSuffix(p, "/") {
		// /0/106 或 /0/106/index.html 都归一化到 /0/106/
		if i := strings.LastIndex(p, "/"); i >= 0 {
			last := p[i+1:]
			if strings.Contains(last, ".") {
				p = p[:i+1]
			} else {
				p = p + "/"
			}
		}
	}

	return p
}

// 解析单页章节目录
// 返回 [章节标题, 绝对链接] 列表；abandon 为需丢弃的章节数（仅首页生效）
func (this *Snatch) parseCatalogPage(doc *goquery.Document, provider *models.SnatchRule, base *url.URL, abandon int) [][2]string {
	rule := provider.Rules
	out := make([][2]string, 0)

	sel := doc.Find(rule.ChapterCatalogSelector)
	size := sel.Size()

	// 新书章节小于丢弃数量情况，防止丢失章节
	if abandon > 0 && size < abandon*2 {
		abandon = size / 2
	}

	skip := 0
	lastChap := ""

	sel.Each(func(i int, s *goquery.Selection) {
		// 过滤掉最新章节
		if skip < abandon {
			skip++
			return
		}

		href, _ := s.Attr("href")
		if len(href) == 0 {
			return
		}

		title := strings.TrimSpace(s.Text())

		// 章节名称去重
		if strings.EqualFold(title, lastChap) {
			return
		}
		lastChap = title

		abs, err := this.genrateURL(base, href)
		if err != nil || len(abs) == 0 {
			return
		}

		out = append(out, [2]string{title, abs})
	})

	return out
}

// 获取小说章节列表
//
// 相对原实现的两处修正：
//  1. 章节编号统一在最后按顺序赋值。原实现在「下一页」递归时 chapNo 会重置为 1，
//     导致第二章节目录起编号与第一页重复。
//  2. 支持「URL 模板分页」。部分站点（如 5566xs）的目录每页 100 章且页面里
//     没有"下一页"链接，只能按 index_1/2/3.html 递增，原实现无法翻页，
//     一本 1453 章的书只能采到前 100 章。
func (this *Snatch) GetChapters(provider *models.SnatchRule, rawurl string) ([]*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}
	t1 := time.Now()

	rule := provider.Rules

	if rule == nil {
		return nil, ErrNotRule
	}

	// 解析URL
	rawurl = strings.TrimSpace(rawurl)
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	// 汇总的章节（标题+链接），并用链接去重
	all := make([][2]string, 0)
	seen := make(map[string]bool)

	addAll := func(items [][2]string) int {
		fresh := 0
		for _, it := range items {
			if seen[it[1]] {
				continue
			}
			seen[it[1]] = true
			all = append(all, it)
			fresh++
		}

		return fresh
	}

	// 第一页（丢弃章节数仅对首页生效）
	doc, _, err := this.newHtml(rawurl, provider.Charset, true)
	if err != nil {
		return make([]*SnatchInfo, 0), err
	}
	addAll(this.parseCatalogPage(doc, provider, u, rule.ChapterAbandonNum))

	// 方式一：URL 模板分页（无"下一页"链接的站点）
	if tpl := strings.TrimSpace(rule.ChapterPageURLTemplate); len(tpl) > 0 {
		start := rule.ChapterPageStart
		if start < 2 {
			// 第 1 页即上面已采集的 base URL
			start = 2
		}

		max := rule.ChapterPageMax
		if max < start {
			// 未配置上限时给一个安全默认值，避免异常规则死循环
			max = start + 99
		}

		// 模板支持两种占位符：
		//   {page}  页码
		//   {book}  书籍目录地址（由当前书籍 URL 推导，如 /0/106/）
		// 这样同一条规则可适用于所有书，无需为每本书单独写规则。
		bookPath := bookDirPath(rawurl)

		for p := start; p <= max; p++ {
			pageURL := strings.Replace(tpl, "{page}", strconv.Itoa(p), 1)
			pageURL = strings.Replace(pageURL, "{book}", bookPath, 1)

			pgDoc, _, err := this.newHtml(pageURL, provider.Charset, true)
			if err != nil {
				log.Warn(fmt.Sprintf("[%s]目录分页采集失败[%s]：%v", provider.Name, pageURL, err))
				break
			}

			fresh := addAll(this.parseCatalogPage(pgDoc, provider, u, 0))

			// 整页没有任何新章节，说明已越过末页
			// （部分站点越界后会回卷到第 1 页，此时全部命中已采集集合）
			if fresh == 0 {
				break
			}

			time.Sleep(CHAPTER_PAGE_INTERVAL)
		}
	} else if len(rule.ChapterNextPageSelector) > 0 {
		// 方式二：选择器式下一页（保留原有能力，改为迭代并带去重/防环）
		cur := rawurl
		curDoc := doc
		for i := 0; i < MAX_CATALOG_PAGES; i++ {
			nextURL := curDoc.Find(rule.ChapterNextPageSelector).AttrOr("href", "")
			if len(nextURL) == 0 {
				break
			}

			nextURL, err = this.genrateURL(u, nextURL)
			if err != nil || len(nextURL) == 0 || nextURL == cur || this.IsBookURL(provider, nextURL) {
				break
			}

			if seen["__page__"+nextURL] {
				break
			}
			seen["__page__"+nextURL] = true

			nextDoc, _, err := this.newHtml(nextURL, provider.Charset, true)
			if err != nil {
				log.Warn(fmt.Sprintf("[%s]目录分页采集失败[%s]：%v", provider.Name, nextURL, err))
				break
			}

			// 丢弃章节数不再对后续分页生效，且不再修改 provider 共享状态
			fresh := addAll(this.parseCatalogPage(nextDoc, provider, u, 0))

			cur = nextURL
			curDoc = nextDoc

			if fresh == 0 {
				break
			}

			time.Sleep(CHAPTER_PAGE_INTERVAL)
		}
	}

	// 统一编号
	links := make([]*SnatchInfo, 0, len(all))
	for i, it := range all {
		chap := models.NewChapter()
		chap.Title = it[0]
		chap.Link = it[1]
		chap.ChapterNo = uint32(i + 1)

		links = append(links, &SnatchInfo{
			Chap:    chap,
			UseTime: time.Since(t1),
		})
	}

	log.Debug(fmt.Sprintf("[%s]获取小说章节列表[%s]，共 %d 章，使用时间：%v", provider.Name, rawurl, len(links), time.Since(t1)))

	return links, nil
}

// 代理设置
func (this *Snatch) Proxy(proxyFunc func() string) {
	this.proxyFunc = proxyFunc
}

// 网页请求，失败重试
// 返回goquery
//
// 相对原实现的三处改进（均针对源站限流）：
//  1. 请求前经过全局节流器：同域名串行 + 最小间隔 + 随机抖动，
//     避免「同一秒内十几个请求打向同一站」而被判定为爬虫；
//  2. 失败重试用指数退避（1s,2s,4s,8s…）替代原先固定 sleep 10ms，
//     原做法在被限流时等于继续猛打，形成重试风暴；
//  3. 识别 429/503 等限流响应，并尊重 Retry-After 头，
//     随后对整个域名做一段冷却，等对方恢复再继续。
func (this *Snatch) newHtml(rawurl, charset string, isRedirect bool) (*goquery.Document, *http.Response, error) {
	var res []byte
	var resp *http.Response
	var body io.Reader
	var err error

	conf := &xhttp.ClientConfig{
		Timeout:   10 * time.Second,
		Dial:      10 * time.Second,
		KeepAlive: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 存在重定向是否直接跳转
			if isRedirect {
				return nil
			}
			return http.ErrUseLastResponse
		},
	}
	c := xhttp.NewClient(conf)

	throttle := utils.ThrottleInstance()
	interval, jitter := snatchPace()

	// 失败重试（指数退避）
	for i := 0; i < RETRY; i++ {
		// 同域名串行 + 最小间隔，防止并发冲击目标站
		throttle.Wait(rawurl, interval, jitter)

		c.SetProxy(this.proxyFunc())
		res, resp, err = c.Get(context.TODO(), rawurl, nil)

		throttle.Done(rawurl)

		if err == nil {
			break
		}

		if resp != nil && (resp.StatusCode == 301 || resp.StatusCode == 302) {
			break
		}

		log.Debug("请求失败：", rawurl, err)

		// 被限流：尊重 Retry-After，并让整个域名冷却
		if resp != nil && isRateLimited(resp.StatusCode) {
			cd := rateLimitCooldown(resp)
			log.Warn(fmt.Sprintf("目标站限流(HTTP %d)，冷却 %v 后继续：%s",
				resp.StatusCode, cd, rawurl))
			throttle.Cooldown(rawurl, cd)
			continue
		}

		// 指数退避：1s、2s、4s、8s…上限 30s
		if i < RETRY-1 {
			backoff := time.Duration(1<<uint(i)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			log.Debug(fmt.Sprintf("第 %d 次重试，等待 %v：%s", i+1, backoff, rawurl))
			time.Sleep(backoff)
		}
	}

	if err != nil {
		return nil, resp, err
	}

	body = bytes.NewReader(res)

	// 编码转换
	if charset != "UTF-8" {
		enc := mahonia.NewDecoder("GB18030")
		body = enc.NewReader(body)
	}

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, resp, err
	}

	return doc, resp, nil
}

// 采集内容过滤
func (this *Snatch) Filter(filter, kw string) string {
	return this.filter(filter, kw)
}

// 采集内容过滤
func (this *Snatch) filter(filter, kw string) string {
	if len(kw) == 0 || len(filter) == 0 {
		return kw
	}

	// 正则过滤关键词
	keyexs := strings.Split(filter, "\n")
	keyexs = append(keyexs, defaultFilterRules...)
	for _, v := range keyexs {
		if len(v) == 0 {
			continue
		}
		replaced := ""
		if strings.Contains(v, " | ") {
			aR := strings.Split(v, " | ")
			v = aR[0]
			replaced = aR[1]
		}

		re, _ := regexp.Compile("(?U)" + v)
		kw = re.ReplaceAllString(kw, replaced)
	}

	return kw
}

// 拼装当前页面获取到的连接，生成完整的URL
func (this *Snatch) genrateURL(base *url.URL, rawurl string) (string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}

	return base.ResolveReference(u).String(), nil
}

// FindNovelList 搜索并返回多条结果
//
// 供后台「搜索采集站」页面聚合展示用。HTML 站点的搜索页通常只给一条
// 最匹配的结果，因此除接口型站点外，其余站点仍只返回一条。
func (this *Snatch) FindNovelList(provider *models.SnatchRule, kw string, limit int) ([]*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	if isDeadBqgnovelsSearch(provider) {
		return this.searchBqgnovelsViaAPI(provider, kw, limit)
	}

	one, err := this.FindNovel(provider, kw)
	if err != nil {
		return nil, err
	}

	return []*SnatchInfo{one}, nil
}

// findNovelByPost 以 POST 方式搜索（供只接受 POST 的站点使用）
//
// 规则写法：POST:{url}|{参数模板}
// 参数模板中用 {{kw}} 占位关键词，以 & 分隔多个参数，值需自行做
// URL 编码（因为要作为表单字段值发送）。
func (this *Snatch) findNovelByPost(provider *models.SnatchRule, rule *models.Rule, charset, kw string) (*SnatchInfo, error) {
	t1 := time.Now()

	spec := strings.TrimPrefix(rule.FindURL, "POST:")

	parts := strings.SplitN(spec, "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("POST 搜索地址格式错误: %s", rule.FindURL)
	}

	searchURL := strings.TrimSpace(parts[0])
	paramTpl := strings.TrimSpace(parts[1])

	// 组装表单参数
	form := url.Values{}
	for _, pair := range strings.Split(paramTpl, "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.ReplaceAll(kv[1], "{{kw}}", kw)
		form.Set(k, v)
	}

	// 请求搜索页
	res, resp, err := this.postForm(searchURL, form, charset)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, ErrNotResp
	}

	// 解析结果页
	doc, err := parseHTML(res, charset)
	if err != nil {
		return nil, err
	}

	// 定位书籍页地址
	novURL := resp.Header.Get("Location")
	if len(novURL) == 0 || !this.IsBookURL(provider, novURL) {
		novURL, _ = doc.Find(rule.FindBookURLSelector).Attr("href")
	}

	if len(novURL) == 0 {
		return nil, ErrNotNovURL
	}

	base, err := url.Parse(searchURL)
	if err != nil {
		return nil, err
	}

	novURL, err = this.genrateURL(base, novURL)
	if err != nil {
		return nil, err
	}

	if !this.IsBookURL(provider, novURL) {
		return nil, ErrInvalidURL
	}

	info, err := this.GetNovel(provider, novURL)
	if err != nil {
		return nil, err
	}

	info.UseTime = time.Since(t1)

	log.Debug(fmt.Sprintf("[%s]查找小说[%s]，使用时间：%v", provider.Name, info.Nov.Name, info.UseTime))

	return info, nil
}

// postForm 发送表单 POST 请求
func (this *Snatch) postForm(rawurl string, form url.Values, charset string) ([]byte, *http.Response, error) {
	conf := &xhttp.ClientConfig{
		Timeout:   30 * time.Second,
		Dial:      15 * time.Second,
		KeepAlive: 60 * time.Second,
	}

	c := xhttp.NewClient(conf)
	if this.proxyFunc != nil {
		c.SetProxy(this.proxyFunc())
	}

	interval, jitter := snatchPace()
	throttle := utils.ThrottleInstance()
	throttle.Wait(rawurl, interval, jitter)
	res, resp, err := c.Post(context.TODO(), rawurl, form, nil)
	throttle.Done(rawurl)

	return res, resp, err
}

// parseHTML 按指定编码解析 HTML
func parseHTML(res []byte, charset string) (*goquery.Document, error) {
	var body io.Reader = bytes.NewReader(res)

	if charset != "" && charset != "UTF-8" {
		if enc := mahonia.NewDecoder("GB18030"); enc != nil {
			body = enc.NewReader(body)
		}
	}

	return goquery.NewDocumentFromReader(body)
}
