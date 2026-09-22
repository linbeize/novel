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

package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego"

	xhttp "github.com/vckai/novel/app/librarys/net/http"
	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services/snatchs"
	"github.com/vckai/novel/app/utils/log"
)

var (
	ErrNotProvider    = errors.New("没有获取到采集点")
	ErrURLSnatchMatch = errors.New("该小说URL匹配不到采集站点")
)

type Snatch struct {
	c *snatchs.Snatch

	// api 接口型采集器（用于 SPA + JSON 接口结构的站点）
	api *snatchs.ApiSnatch
}

func NewSnatch() *Snatch {
	// 注入采集节奏来源：读取后台可配置的间隔，便于随时调速而无需改代码
	// 采集节奏：单位毫秒，未配置时用默认值
	pace := func() (time.Duration, time.Duration) {
		interval := ConfigService.Int64("SnatchInterval", 1500)
		jitter := ConfigService.Int64("SnatchJitter", 800)

		if interval < 0 {
			interval = 0
		}
		if jitter < 0 {
			jitter = 0
		}

		return time.Duration(interval) * time.Millisecond,
			time.Duration(jitter) * time.Millisecond
	}

	// 代理来源
	proxy := func() string {
		return ProxyService.Get()
	}

	snatchs.SetPaceProvider(pace)
	snatchs.SetApiProxyProvider(proxy)

	// 接口型采集器（bqglll）的可调参数：读取后台「采集源设置」中的配置。
	// 每次调用实时读取，故后台改完即时生效，无需重启。
	// 投毒判定的阈值同样由后台设置控制
	snatchs.SetPoisonThresholdProvider(func() int {
		d := snatchs.DefaultApiSettings()
		return int(ConfigService.Int64("BqglllPoisonThreshold", int64(d.PoisonThreshold)))
	})

	snatchs.SetApiSettingsProvider(func() snatchs.ApiSettings {
		d := snatchs.DefaultApiSettings()

		get := func(key string, def int) int {
			return int(ConfigService.Int64(key, int64(def)))
		}

		return snatchs.ApiSettings{
			IntervalMs:      get("BqglllInterval", d.IntervalMs),
			JitterMs:        get("BqglllJitter", d.JitterMs),
			TimeoutSec:      get("BqglllTimeout", d.TimeoutSec),
			Retry:           get("BqglllRetry", d.Retry),
			MaxDesc:         get("BqglllMaxDesc", d.MaxDesc),
			DefaultCate:     uint32(get("BqglllDefaultCate", int(d.DefaultCate))),
			PoisonCheck:     ConfigService.Bool("BqglllPoisonCheck", d.PoisonCheck),
			PoisonThreshold: get("BqglllPoisonThreshold", d.PoisonThreshold),
		}
	})

	return &Snatch{
		c: snatchs.NewSnatch(func() string {
			return ProxyService.Get()
		}),
		api: snatchs.NewApiSnatch(),
	}
}

// 判断该规则是否走接口型采集
func (this *Snatch) isAPI(provider *models.SnatchRule) bool {
	return snatchs.IsAPI(provider)
}

// 执行指定小说的采集任务
func (this *Snatch) SnatchNovel(novId uint32) error {
	return manager.RunOneTask(novId)
}

// 是否小说简介页面
func (this *Snatch) IsBookURL(source, rawurl string) bool {
	provider := SnatchRuleService.GetByCode(source)
	return this.c.IsBookURL(provider, rawurl)
}

// 是否小说是否爬虫页面
func (this *Snatch) IsCrawlerURL(source, rawurl string) bool {
	provider := SnatchRuleService.GetByCode(source)
	return this.c.IsCrawlerURL(provider, rawurl)
}

// 查找小说列表（跨站聚合搜索）
//
// 给出各采集站的结果供挑选。相比早先的实现有三点改动：
//
//  1. 并行搜索：各站互不依赖，串行时总耗时等于各站之和
//     （实测 6 个站约 20-40 秒），并行后取决于最慢的那个站。
//
//  2. 每站返回多条：只有第一条时常漏掉用户真正要的那本
//     （搜「斗破苍穹」第一条可能是《斗破苍穹之召唤帝》），
//     故改为返回该站的前若干条。
//
//  3. 记录站点名：原实现只填了 Source（代号），页面上想显示
//     「来自哪个站」时取不到中文名，这里一并填入。
func (this *Snatch) FindNovels(kw string) []*snatchs.SnatchInfo {
	// 只搜索已启用的站点：失效/停用的规则搜了也是白耗时间
	allProviders := SnatchRuleService.GetSnatchs()

	providers := make([]*models.SnatchRule, 0, len(allProviders))
	for _, p := range allProviders {
		if p.State == 1 {
			providers = append(providers, p)
		}
	}

	if len(providers) == 0 {
		return nil
	}

	// 每个站最多取多少条
	const perSite = 20

	chans := make([]chan resultSet, 0, len(providers))

	for _, provider := range providers {
		ch := make(chan resultSet, 1)
		chans = append(chans, ch)

		go func(p *models.SnatchRule) {
			// 单站限时：站点失效时会长时间等待连接超时（实测 18-19 秒），
			// 而搜索是并行进行的，总耗时取决于最慢的站。
			// 这里给每个站设上限，超时即放弃该站结果。
			done := make(chan resultSet, 1)

			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Warn("搜索采集站异常:", p.Code, " ", r)
						done <- resultSet{}
					}
				}()
				done <- searchOneSite(this, p, kw, perSite)
			}()

			select {
			case r := <-done:
				ch <- r
			case <-time.After(searchTimeout):
				log.Debug("搜索站点超时:", p.Code)
				ch <- resultSet{}
			}
		}(provider)
	}

	var list []*snatchs.SnatchInfo
	for _, ch := range chans {
		r := <-ch
		list = append(list, r.list...)
	}

	return list
}

// searchTimeout 单个站点的搜索时限
//
// 取 8 秒：正常站点实测 1-4 秒，留有余量；
// 失效站点（如已停运的 biqtxt，连接超时需 18-19 秒）则被截断。
const searchTimeout = 8 * time.Second

// resultSet 单站搜索结果
type resultSet struct {
	list []*snatchs.SnatchInfo
}

// searchOneSite 搜索单个站点（供 FindNovels 并行调用）
func searchOneSite(svc *Snatch, p *models.SnatchRule, kw string, perSite int) resultSet {
	var (
		items []*snatchs.SnatchInfo
		err   error
	)

	if svc.isAPI(p) {
		items, err = svc.api.FindNovelList(p, kw, perSite)
	} else {
		// HTML 站点的搜索页通常只给一条最匹配的结果，
		// 但改版站点（如 bqgnovels）已切到 JSON 接口，
		// 由 FindNovelList 内部判断并可能返回多条。
		items, err = svc.c.FindNovelList(p, kw, perSite)
	}

	if err != nil {
		log.Debug("搜索站点无结果:", p.Code, " ", err.Error())
		return resultSet{}
	}

	// 补站点名，便于页面展示
	for _, it := range items {
		it.SiteName = p.Name
	}

	return resultSet{list: items}
}

// 查找小说
func (this *Snatch) FindNovel(source, kw string) (*snatchs.SnatchInfo, error) {
	provider := SnatchRuleService.GetByCode(source)
	if this.isAPI(provider) {
		return this.api.FindNovel(provider, kw)
	}
	return this.c.FindNovel(provider, kw)
}

// 获取一本小说
func (this *Snatch) GetNovel(source, rawurl string) (*snatchs.SnatchInfo, error) {
	provider := SnatchRuleService.GetByCode(source)
	if this.isAPI(provider) {
		return this.api.GetNovel(provider, rawurl)
	}
	return this.c.GetNovel(provider, rawurl)
}

// 获取小说章节内容
func (this *Snatch) GetChapter(source, rawurl string) (*snatchs.SnatchInfo, error) {
	provider := SnatchRuleService.GetByCode(source)
	if this.isAPI(provider) {
		return this.api.GetChapter(provider, rawurl)
	}
	return this.c.GetChapter(provider, rawurl)
}

// 获取小说章节内容（自动拼接同章分页正文）
func (this *Snatch) GetChapterFull(source, rawurl string) (*snatchs.SnatchInfo, error) {
	provider := SnatchRuleService.GetByCode(source)
	if this.isAPI(provider) {
		return this.api.GetChapterFull(provider, rawurl)
	}
	return this.c.GetChapterFull(provider, rawurl)
}

// 获取小说章节列表
func (this *Snatch) GetChapters(source, rawurl string) ([]*snatchs.SnatchInfo, error) {
	provider := SnatchRuleService.GetByCode(source)
	if this.isAPI(provider) {
		return this.api.GetChapters(provider, rawurl)
	}
	return this.c.GetChapters(provider, rawurl)
}

// 根据URL匹配采集站点
func (this *Snatch) GetProvideByURL(url string) (*models.SnatchRule, error) {
	providers := SnatchRuleService.GetSnatchs()
	for _, v := range providers {
		// 匹配采集站点成功
		if this.c.IsBookURL(v, url) {
			return v, nil
		}
	}

	return nil, ErrURLSnatchMatch
}

// 初始化一本小说
// 采集小说入库
func (this *Snatch) InitNovel(url string) error {
	// 获取小说采集器
	provider, err := this.GetProvideByURL(url)
	if err != nil {
		return err
	}

	// 获取小说（按规则类型分派到对应的采集器）
	var info *snatchs.SnatchInfo
	if this.isAPI(provider) {
		info, err = this.api.GetNovel(provider, url)
	} else {
		info, err = this.c.GetNovel(provider, url)
	}
	if err != nil {
		return err
	}

	nov := info.Nov

	// 判断小说是否已存在
	nv := NovelService.GetByName(nov.Name)

	// 下载封面图片
	isUp := false
	if len(nov.Cover) != 0 && (len(nv.Cover) == 0 || provider.IsUpdate == 1) {
		imgFile, err := DownImg(nov.Cover)
		if err != nil {
			log.Warn("下载图片失败：", nov.Cover, err)
			nov.Cover = ""
		} else {
			nov.Cover = ConfigService.String("ViewURL") + imgFile
		}
		isUp = true
		nv.Cover = nov.Cover
	}

	// 更新小说简介
	if nov.Desc != nv.Desc && provider.IsUpdate == 1 {
		nv.Desc = nov.Desc
		isUp = true
	}

	// 更新小说作者名称
	if nv.Author == "" {
		nv.Author = nov.Author
		isUp = true
	}

	if nv.Id > 0 {
		// 添加采集点即可
		if info.Url != "" {
			NovelService.AddLink(nv.Id, info.Url, provider.Code, info.ChapterUrl)
		}

		// 更新小说简介内容和封面图片
		if isUp {
			nv.CateId = nov.CateId
			err = NovelService.UpNovelInfo(nv)
		}

		return err
	}

	// 保存小说
	err = NovelService.Save(nov)
	if err != nil {
		return err
	}

	// 添加采集URL
	if info.Url != "" {
		NovelService.AddLink(nov.Id, info.Url, provider.Code, info.ChapterUrl)
	}

	// 添加到采集队列中
	manager.AddTask(nov.Id)

	return nil
}

// 下载缩略图
func DownImg(rawurl string) (string, error) {
	fileType := rawurl[strings.LastIndex(rawurl, "."):]
	if fileType != ".jpeg" && fileType != ".png" && fileType != ".jpg" {
		fileType = ".jpeg"
	}
	upPreDir := beego.AppConfig.String("static::uppredir")
	upDir := beego.AppConfig.String("static::updir")

	// 重命名文件
	newName := strconv.FormatInt(time.Now().UnixNano(), 10) + fileType

	// 创建上传目录
	uploadDir := upDir + time.Now().Format("2006/01/02") + "/"

	err := os.MkdirAll(upPreDir+uploadDir, os.ModePerm) //创建目录
	if err != nil {
		return "", err
	}

	c := xhttp.NewClient(
		&xhttp.ClientConfig{
			Timeout:   10 * time.Second,
			Dial:      500 * time.Millisecond,
			KeepAlive: 60 * time.Second,
			ProxyURL:  ProxyService.Get(),
		})

	body, _, err := c.Get(context.TODO(), rawurl, nil)
	if err != nil {
		return "", err
	}

	// 创建空文件
	f, err := os.Create(upPreDir + uploadDir + newName)
	if err != nil {
		return "", err
	}

	defer f.Close()

	rc := bytes.NewReader(body)

	_, err = io.Copy(f, rc)

	return uploadDir + newName, err
}

// CrawlCategories 批量采集某站点的分类页（仅接口型采集器支持）
//
// 用于 SPA 站点：其分类页内容由前端 JS 从接口加载，通用 HTML 爬虫
// 抓不到。返回值为发现的书籍数。
func (this *Snatch) CrawlCategories(source string, found func(link string) bool) int {
	provider := SnatchRuleService.GetByCode(source)
	if provider == nil {
		return 0
	}
	if !snatchs.IsAPI(provider) {
		return 0
	}

	return this.api.CrawlCategories(found)
}
