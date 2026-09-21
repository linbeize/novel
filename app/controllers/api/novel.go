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

package api

import (
	"html"
	"regexp"
	"strings"

	"github.com/astaxie/beego"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services"
)

/*
只读浏览接口

路由（均在 /api 命名空间下，与站内页面路由互不影响）：

	GET /api/home                 首页聚合（推荐、榜单、最新更新）
	GET /api/cates                分类列表
	GET /api/books                书籍列表（分类 / 状态 / 字数 / 排序 / 关键词）
	GET /api/search               搜索（/api/books 的别名，语义更直观）
	GET /api/book/:id             书籍详情
	GET /api/book/:id/chapters    章节目录（分页）
	GET /api/chapter/:id          章节正文
	GET /api/rank                 排行榜

字段命名统一用小写 + 下划线，与其他接口风格一致。
时间统一返回 Unix 时间戳与格式化字符串两种，客户端按需取用。
*/

// 首页聚合
//
// 一次返回 App 首屏所需的全部数据，减少请求数。
func (this *NovelController) Home() {
	theme := services.ConfigService.String("Theme")

	// 轮播：与网页版首页保持一致，取「PC 首页轮播」区域
	banners := services.BannerService.GetAll(map[string]interface{}{
		"zone": int(services.ZONEINDEXSLICE),
	})

	bannerList := make([]map[string]interface{}, 0, len(banners))
	for _, b := range banners {
		bannerList = append(bannerList, map[string]interface{}{
			"id":    b.Id,
			"name":  b.Name,
			"cover": this.absURL(b.Img),
			"link":  b.Link,
			"desc":  b.Desc,
		})
	}

	this.ok(map[string]interface{}{
		"theme":   theme,
		"banners": bannerList,
		// 今日推荐
		"today_recs": this.novelBriefList(services.NovelService.GetTodayRecs(6, 0)),
		// 编辑推荐
		"recs": this.novelBriefList(services.NovelService.GetRecs(12, 0)),
		// 热门
		"hots": this.novelBriefList(services.NovelService.GetHots(12, 0)),
		// 最新更新
		"new_ups": this.novelBriefList(services.NovelService.GetNewUps(12, 0)),
		// 排行榜
		"ranks": this.novelBriefList(services.NovelService.GetRanks(10, 0)),
	})
}

// 分类列表
func (this *NovelController) Cates() {
	cates := services.CateService.GetAll()

	list := make([]map[string]interface{}, 0, len(cates))
	for _, c := range cates {
		// 只返回在菜单中展示的分类，与前台导航保持一致
		if c.IsMenu != 1 {
			continue
		}
		list = append(list, map[string]interface{}{
			"id":   c.Id,
			"name": c.Name,
		})
	}

	this.ok(map[string]interface{}{
		"list":  list,
		"total": len(list),
	})
}

// 书籍列表 / 搜索
//
// 参数：
//
//	cate      分类 ID
//	status    状态：1 连载中、2 已完结
//	text_num  字数区间：1..5（与前台筛选一致）
//	uptime    更新时间：1..4（3天/7天/15天/30天内）
//	ot        排序：1 人气、2 更新、3 字数
//	kw        关键词（按书名搜索）
//	p         页码，默认 1
//	size      每页条数，默认 20，上限 100
func (this *NovelController) Books() {
	page, size, offset := this.pageParams()

	cateId, _ := this.GetInt("cate", 0)
	status, _ := this.GetInt("status", 0)
	textNum, _ := this.GetInt("text_num", 0)
	upTime, _ := this.GetInt("uptime", 0)
	ot, _ := this.GetInt("ot", 1)
	kw := strings.TrimSpace(this.GetString("kw"))

	search := map[string]interface{}{
		"count":    true,
		"cate_id":  cateId,
		"status":   status,
		"text_num": textNum,
		"uptime":   upTime,
		"ot":       ot,
	}

	if kw != "" {
		search["q"] = kw
	}

	novs, count := services.NovelService.GetList(size, offset, search)

	// service 层为网页搜索做了关键词高亮，会往书名里插入
	// <font color="red"> 标签。API 返回的是纯数据，需去掉。
	for _, n := range novs {
		n.Name = stripHighlight(n.Name)
	}

	this.ok(map[string]interface{}{
		"list":  this.novelBriefList(novs),
		"total": count,
		"page":  page,
		"size":  size,
		"pages": this.pages(count, size),
	})
}

// 搜索（与 Books 同实现，仅路径更直观）
func (this *NovelController) Search() {
	this.Books()
}

// 排行榜
//
// 参数：cate 可选，指定分类则返回该分类的热门。
func (this *NovelController) Rank() {
	cateId, _ := this.GetInt("cate", 0)

	var novs []*models.Novel
	if cateId > 0 {
		novs = services.NovelService.GetCateRanks(cateId, 20, 0)
	} else {
		novs = services.NovelService.GetRanks(20, 0)
	}

	this.ok(map[string]interface{}{
		"list": this.novelBriefList(novs),
	})
}

// 书籍详情
func (this *NovelController) Detail() {
	id, _ := this.GetUint32(":id")
	if id < 1 {
		this.fail(CodeParamErr, "参数错误")
		return
	}

	nov := services.NovelService.Get(id)
	if nov == nil {
		this.fail(CodeNotFound, "该小说不存在或者已被删除")
		return
	}

	// 累计浏览次数（与网页版行为一致）
	services.NovelService.UpViews(nov.Id)

	// 首章，供「开始阅读」直接跳转
	firstChapId := uint64(0)
	if first := services.ChapterService.GetFirst(nov.Id); first != nil {
		firstChapId = first.Id
	}

	this.ok(map[string]interface{}{
		"novel":         this.novelDetail(nov),
		"first_chap_id": firstChapId,
	})
}

// 章节目录
//
// 参数：p 页码、size 每页条数、order 排序（asc 正序 / desc 倒序）
func (this *NovelController) Chapters() {
	novId, _ := this.GetUint32(":id")
	if novId < 1 {
		this.fail(CodeParamErr, "参数错误")
		return
	}

	nov := services.NovelService.Get(novId)
	if nov == nil {
		this.fail(CodeNotFound, "该小说不存在或者已被删除")
		return
	}

	page, size, offset := this.pageParams()

	// 章节默认正序；书籍详情页的目录一般倒序展示最新，故支持切换
	order := strings.ToLower(this.GetString("order", "asc"))
	if order != "desc" {
		order = "asc"
	}

	chaps, count := services.ChapterService.GetNovChaps(novId, size, offset, order, true)

	list := make([]map[string]interface{}, 0, len(chaps))
	for _, c := range chaps {
		list = append(list, map[string]interface{}{
			"id":         c.Id,
			"chapter_no": c.ChapterNo,
			"title":      c.Title,
			"text_num":   c.TextNum,
			"updated_at": c.UpdatedAt,
		})
	}

	this.ok(map[string]interface{}{
		"novel_id":   novId,
		"novel_name": nov.Name,
		"list":       list,
		"total":      count,
		"page":       page,
		"size":       size,
		"pages":      this.pages(count, size),
	})
}

// 章节正文
//
// 参数：novid 小说 ID（必填，章节分表按小说号定位，缺少则查不到）
//
//	with_nav 是否附带上下章 ID，默认 1
func (this *NovelController) Chapter() {
	id, _ := this.GetUint64(":id")
	novId, _ := this.GetUint32("novid")
	if id < 1 || novId < 1 {
		this.fail(CodeParamErr, "参数错误：需要 id 与 novid")
		return
	}

	chap := services.ChapterService.Get(id, novId)
	if chap == nil {
		this.fail(CodeNotFound, "该章节不存在或者已被删除")
		return
	}

	data := map[string]interface{}{
		"id":         chap.Id,
		"novel_id":   chap.NovId,
		"chapter_no": chap.ChapterNo,
		"title":      chap.Title,
		// 正文为采集到的 HTML 片段，客户端可直接渲染或自行转纯文本
		"content":    chap.Desc,
		"text_num":   chap.TextNum,
		"created_at": chap.CreatedAt,
	}

	if this.GetString("with_nav", "1") == "1" {
		nextId := uint64(0)
		if next := services.ChapterService.GetNext(chap.NovId, chap.ChapterNo); next != nil {
			nextId = next.Id
		}

		preId := uint64(0)
		if pre := services.ChapterService.GetPre(chap.NovId, chap.ChapterNo); pre != nil {
			preId = pre.Id
		}

		data["next_id"] = nextId
		data["pre_id"] = preId
	}

	this.ok(data)
}

/* ---------- 内部辅助 ---------- */

// 书籍简要信息（列表用）
func (this *BaseController) novelBriefList(novs []*models.Novel) []map[string]interface{} {
	list := make([]map[string]interface{}, 0, len(novs))
	for _, n := range novs {
		list = append(list, map[string]interface{}{
			"id":                 n.Id,
			"name":               n.Name,
			"author":             n.Author,
			"cover":              this.absURL(n.Cover),
			"cate_id":            n.CateId,
			"cate_name":          n.CateName,
			"status":             n.Status,
			"status_name":        n.StatusName(),
			"text_num":           n.TextNum,
			"chapter_num":        n.ChapterNum,
			"chapter_title":      n.ChapterTitle,
			"chapter_updated_at": n.ChapterUpdatedAt,
			"views":              n.Views,
		})
	}
	return list
}

// 书籍详细信息（详情页用）
func (this *BaseController) novelDetail(n *models.Novel) map[string]interface{} {
	return map[string]interface{}{
		"id":                 n.Id,
		"name":               n.Name,
		"author":             n.Author,
		"cover":              this.absURL(n.Cover),
		"desc":               this.plainText(n.Desc),
		"cate_id":            n.CateId,
		"cate_name":          n.CateName,
		"status":             n.Status,
		"status_name":        n.StatusName(),
		"text_num":           n.TextNum,
		"chapter_num":        n.ChapterNum,
		"chapter_id":         n.ChapterId,
		"chapter_title":      n.ChapterTitle,
		"chapter_updated_at": n.ChapterUpdatedAt,
		"views":              n.Views,
		"created_at":         n.CreatedAt,
		"updated_at":         n.UpdatedAt,
		"is_original":        n.IsOriginal,
	}
}

// 把简介的 HTML 片段转为纯文本
//
// 库中 desc 可能含 <br>、&nbsp; 等标记。这里不做截断——App 需要完整简介，
// 由客户端自行决定展示多少。
func (this *BaseController) plainText(s string) string {
	if s == "" {
		return ""
	}

	s = beego.HTML2str(s)
	s = strings.Replace(s, "\u00a0", " ", -1)
	s = strings.Replace(s, "\n", " ", -1)

	return strings.TrimSpace(s)
}

// 计算总页数
func (this *BaseController) pages(total int64, size int) int {
	if size < 1 || total <= 0 {
		return 0
	}
	return int((total + int64(size) - 1) / int64(size))
}

// 去掉搜索关键词的高亮标签
//
// service 层为了让网页搜索结果高亮，会把书名里的关键词包成
// <font color="red">关键词</font>。客户端只需纯文本，故此处剥离标签。
func stripHighlight(s string) string {
	if !strings.Contains(s, "<font") {
		return s
	}
	return html.UnescapeString(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, ""))
}
