package snatchs

import (
	"fmt"
	"strings"
	"time"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services/snatchs/bqglll"
	"github.com/vckai/novel/app/utils/log"
)

/*
API 型采集器（bqglll.cc）
========================

背景
----
项目原有的采集器是「CSS 选择器 + HTML 解析」，适用于服务端渲染的站点。
而 bqglll.cc 是 SPA + JSON 接口结构：

  - 正文不在 HTML 里，必须调 /api/chapter 获取；
  - 该接口要求 AES 加密的 token，CSS 规则无法表达；
  - 元数据、目录、搜索也走 JSON 接口。

因此无法用现有的规则字段描述，需单独的采集实现。

接入方式
--------
规则表新增 `api_type` 字段（见 models.Rule）。当该字段非空时，
本文件的 ApiSnatch 接管，按接口协议采集；否则仍走原有的
CSS 采集逻辑。这样两种采集器共存，互不影响。

域名与 ID
---------
该站存在双轨 ID：
  - 内部 ID（如 113680）：接口与封面图使用
  - SEO ID（如 104952）：静态页 URL 使用

采集点统一记录内部 ID，形如 `bqglll://113680`，避免与 SEO ID 混淆。
*/

// ApiSnatch API 型采集器
type ApiSnatch struct {
	client *bqglll.Client
}

func NewApiSnatch() *ApiSnatch {
	return &ApiSnatch{client: bqglll.NewClient()}
}

// IsAPI 判断该规则是否走 API 采集
func IsAPI(provider *models.SnatchRule) bool {
	if provider == nil || provider.Rules == nil {
		return false
	}
	return strings.TrimSpace(provider.Rules.APIType) != ""
}

/* ---------- URL 约定 ---------- */

// API 采集点的地址形如 bqglll://113680
const apiScheme = "bqglll://"

// buildLink 由书籍 ID 生成采集点地址
func buildLink(id string) string {
	return apiScheme + id
}

// parseLink 从采集点地址解析书籍 ID
//
// 兼容三种写法，便于手工录入：
//
//	bqglll://113680
//	113680
//	https://www.bqglll.cc/look/104952/   （SEO 页，需先用 API 搜索换到内部 ID）
func parseLink(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("采集点为空")
	}

	if strings.HasPrefix(s, apiScheme) {
		id := strings.TrimPrefix(s, apiScheme)
		if id == "" {
			return "", fmt.Errorf("采集点缺少书籍 ID")
		}
		return id, nil
	}

	// 纯数字
	if isDigits(s) {
		return s, nil
	}

	// SEO 页地址：/look/{seoId}/ ，无法直接用于接口
	if strings.Contains(s, "/look/") {
		return "", fmt.Errorf("该地址是 SEO 页面，请改用内部 ID（可用搜索功能获取）")
	}

	return "", fmt.Errorf("无法识别的采集点: %s", s)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

/* ---------- 采集接口 ---------- */

// FindNovel 搜索小说
func (this *ApiSnatch) FindNovel(provider *models.SnatchRule, kw string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	books, err := this.client.Search(kw)
	if err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return nil, fmt.Errorf("未搜索到小说: %s", kw)
	}

	// 取第一条（与其它采集器行为一致）
	b := books[0]

	nov := models.NewNovel()
	nov.Name = b.Title
	nov.Author = b.Author
	nov.Desc = b.Intro

	// 搜索接口只返回 id/title/author/intro，缺少分类、状态、最新章节，
	// 故再取一次详情补齐。失败不影响搜索结果的返回。
	if detail, derr := this.client.GetBook(b.Id); derr == nil {
		nov.CateName = detail.SortName
		nov.ChapterTitle = detail.LastChapter
		if detail.Full == "完结" || detail.Full == "完本" {
			nov.Status = models.BOOKFINISH
		} else {
			nov.Status = models.BOOKOPEN
		}
	} else {
		log.Warn("获取书籍详情失败（搜索结果仍可用）：", b.Id, " ", derr)
	}

	// 封面：bookimg/{floor(id/1000)}/{id}.jpg
	nov.Cover = coverURL(b.Id)

	return &SnatchInfo{
		Nov:        nov,
		Url:        buildLink(b.Id),
		Source:     provider.Code,
		ChapterUrl: buildLink(b.Id),
	}, nil
}

// GetNovel 取书籍详情
func (this *ApiSnatch) GetNovel(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	id, err := parseLink(rawurl)
	if err != nil {
		return nil, err
	}

	b, err := this.client.GetBook(id)
	if err != nil {
		return nil, err
	}

	nov := models.NewNovel()
	nov.Name = b.Title
	nov.Author = b.Author
	nov.CateName = b.SortName
	nov.Desc = b.Intro
	nov.ChapterTitle = b.LastChapter
	nov.Cover = coverURL(b.Id)

	// 完本状态
	if b.Full == "完结" || b.Full == "完本" {
		nov.Status = models.BOOKFINISH
	} else {
		nov.Status = models.BOOKOPEN
	}

	return &SnatchInfo{
		Nov:        nov,
		Url:        buildLink(b.Id),
		Source:     provider.Code,
		ChapterUrl: buildLink(b.Id),
	}, nil
}

// GetChapters 取章节目录
//
// 接口返回章节标题数组，索引自 1 起（chapterid 即数组下标）。
// 这里统一编号为 1..N，与其它采集器的行为保持一致。
func (this *ApiSnatch) GetChapters(provider *models.SnatchRule, rawurl string) ([]*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	id, err := parseLink(rawurl)
	if err != nil {
		return nil, err
	}

	t1 := time.Now()

	// 目录必须用 dirid（目录 ID），而非书籍 ID。
	//
	// 该站接口存在一个不一致：/api/book 同时返回 id 与 dirid，
	// 而 /api/booklist 的 id 参数实际取的是 dirid。
	// 多数书的 dirid 恰好等于 id，因此这个差异容易漏掉；
	// 但抽样统计约三分之一的书籍两者不同，用错就拿不到目录
	// （接口返回 {"list":null}）。
	// 正文接口 /api/chapter 则两个 ID 都能用，故此处仅目录需要换 ID。
	dirId := id
	if b, derr := this.client.GetBook(id); derr == nil {
		if d := strings.TrimSpace(b.DirId.String()); d != "" && d != id {
			dirId = d
			log.Debug(fmt.Sprintf("[%s]目录 ID 与书籍 ID 不同: book=%s dir=%s",
				provider.Name, id, dirId))
		}
	} else {
		log.Warn("获取 dirid 失败，按书籍 ID 尝试目录：", id, " ", derr)
	}

	titles, err := this.client.GetBookList(dirId)
	if err != nil {
		return nil, err
	}

	// 兼容接口返回 {"list":null} 的情况：换个 ID 再试一次
	if len(titles) == 0 && dirId != id {
		log.Warn("目录为空，回退用书籍 ID 重试：", id)
		titles, _ = this.client.GetBookList(id)
	}
	if len(titles) == 0 {
		return nil, fmt.Errorf("目录为空: id=%s dirid=%s", id, dirId)
	}

	links := make([]*SnatchInfo, 0, len(titles))
	for i, title := range titles {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}

		chap := models.NewChapter()
		chap.Title = title
		chap.ChapterNo = uint32(i + 1)
		// 章节地址：id + 序号，供 GetChapter 解析
		chap.Link = fmt.Sprintf("%s%s/%d", apiScheme, id, i+1)

		links = append(links, &SnatchInfo{
			Chap:    chap,
			UseTime: time.Since(t1),
		})
	}

	log.Debug(fmt.Sprintf("[%s]获取小说章节列表[%s]，共 %d 章，使用时间：%v",
		provider.Name, rawurl, len(links), time.Since(t1)))

	return links, nil
}

// GetChapter 取章节正文
//
// 地址形如 bqglll://113680/688，第二段为章节序号。
func (this *ApiSnatch) GetChapter(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	bookId, chapNo, err := parseChapterLink(rawurl)
	if err != nil {
		return nil, err
	}

	ch, err := this.client.GetChapter(bookId, chapNo)
	if err != nil {
		return nil, err
	}

	chap := models.NewChapter()
	chap.Title = strings.TrimSpace(ch.ChapterName)
	chap.NovId = uint32(bookId)
	chap.ChapterNo = uint32(chapNo)
	// 正文按 \n 分段，这里转为 HTML 片段以与其它采集器一致
	// （入库格式统一为含 <br/> 的片段，前台模板按 HTML 渲染）
	chap.Desc = textToHTML(ch.Txt)
	chap.Link = rawurl
	chap.Source = provider.Code
	chap.TextNum = uint32(len([]rune(ch.Txt)))

	return &SnatchInfo{
		Chap:   chap,
		Url:    rawurl,
		Source: provider.Code,
	}, nil
}

// GetChapterFull 取完整章节正文
//
// 该站的正文接口一次返回整章（前端的分页是纯客户端行为），
// 因此无需像 HTML 采集那样再拼接分页，直接复用 GetChapter 即可。
func (this *ApiSnatch) GetChapterFull(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	return this.GetChapter(provider, rawurl)
}

/* ---------- 内部辅助 ---------- */

// parseChapterLink 解析章节采集点，返回书籍 ID 与章节序号
func parseChapterLink(rawurl string) (int, int, error) {
	s := strings.TrimSpace(rawurl)
	s = strings.TrimPrefix(s, apiScheme)

	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("章节地址格式错误: %s", rawurl)
	}

	bookId := bqglll.IDToInt(parts[0])
	chapNo := bqglll.IDToInt(parts[1])

	if bookId <= 0 || chapNo <= 0 {
		return 0, 0, fmt.Errorf("章节地址缺少有效 ID: %s", rawurl)
	}

	return bookId, chapNo, nil
}

// coverURL 按站点规则拼出封面地址
//
// 规则：bookimg/{floor(id/1000)}/{id}.jpg
func coverURL(id string) string {
	n := bqglll.IDToInt(id)
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("https://www.bqglll.cc/bookimg/%d/%d.jpg", n/1000, n)
}

// textToHTML 把纯文本正文转为与其它采集器一致的 HTML 片段
//
// 站点接口返回的 txt 以 \n 分段。项目内其它采集器的正文均为
// 含 <br/> 的 HTML 片段，前台模板按 HTML 渲染，故此处统一格式。
func textToHTML(txt string) string {
	lines := strings.Split(strings.ReplaceAll(txt, "\r\n", "\n"), "\n")

	var sb strings.Builder
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		// 转义 HTML 特殊字符，避免正文中的尖括号被当作标签
		ln = htmlEscape(ln)
		sb.WriteString("　　")
		sb.WriteString(ln)
		sb.WriteString("<br/>")
	}

	return sb.String()
}

// htmlEscape 转义 HTML 特殊字符
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

/* ---------- 依赖注入（避免包间循环引用） ---------- */

// SetApiPaceProvider 注入采集节奏提供者
//
// 与 CSS 采集器共用同一份节奏配置，保证两种采集器对目标站的行为一致。
func SetApiPaceProvider(f func() (time.Duration, time.Duration)) {
	bqglll.SetPaceProvider(f)
}

// SetApiProxyProvider 注入代理提供者
func SetApiProxyProvider(f func() string) {
	bqglll.SetProxyProvider(f)
}
