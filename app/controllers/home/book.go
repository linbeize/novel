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

package home

import (
	"github.com/astaxie/beego"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services"
	"github.com/vckai/novel/app/utils"
)

type BookController struct {
	BaseController
}

// 目录每页显示多少章
//
// 《地球人实在太凶猛了》这类长篇有近 2000 章，若一次性渲染，详情页会达到
// 500KB 以上（实测 519KB / 0.25s）。分页后单页体积与渲染耗时都大幅下降。
const CATALOG_PAGE_SIZE = 100

// 首页
func (this *BookController) Index() {
	id, _ := this.GetUint32("id")
	if id < 1 {
		this.Msg("参数错误，无法访问")
	}

	// 获取小说详情
	novel := services.NovelService.Get(id)
	if novel == nil {
		this.Msg("该小说不存在或者已被删除")
	}

	// 目录分页：第几页
	page, _ := this.GetInt("p", 1)
	if page < 1 {
		page = 1
	}

	// 该小说章节总数（用于分页器与首章定位）
	total := int(novel.ChapterNum)
	if total < 0 {
		total = 0
	}

	// 页码越界时收敛到最后一页，避免出现空白目录
	maxPage := 1
	if total > 0 {
		maxPage = (total + CATALOG_PAGE_SIZE - 1) / CATALOG_PAGE_SIZE
	}
	if page > maxPage {
		page = maxPage
	}

	// 按页取章节列表
	offset := (page - 1) * CATALOG_PAGE_SIZE
	chaps, _ := services.ChapterService.GetNovChaps(novel.Id, CATALOG_PAGE_SIZE, offset, "asc", false)

	// 目录分页器
	// 注入伪静态地址生成器，使分页链接输出 /book/805/p3.html 形式
	// （而非 /book/805.html?p=3），与站内其它伪静态地址保持一致。
	// 同时用 SetPage 显式告知当前页码：伪静态把页码放在路径中，
	// 分页器默认只读 query 的 p，不设置会导致高亮与前后页判断错误。
	pager := this.SetPaginator(CATALOG_PAGE_SIZE, int64(total))
	pager.SetPage(page)
	novId := novel.Id
	pager.URLBuilder = func(pg int) string {
		return utils.PrettyBookCatalogURL("", int(novId), pg)
	}
	this.Data["CatalogPage"] = page
	this.Data["CatalogTotalPage"] = maxPage

	// 第一章节ID（用于「开始阅读」，需取整本的第一章，而非当前页首章）
	firstChapId := 0
	firstChap := services.ChapterService.GetFirst(novel.Id)
	if firstChap != nil && firstChap.Id > 0 {
		firstChapId = int(firstChap.Id)
	}
	this.Data["FirstChapId"] = firstChapId

	// 获取同类推荐
	this.Data["NovCateRanks"] = services.NovelService.GetCateRanks(int(novel.CateId), 6, 0)

	this.Data["Nov"] = novel
	this.Data["Chaps"] = chaps
	this.Data["Title"] = novel.Name + " - " + novel.Author

	// 获取菜单分类
	this.Data["Cates"] = services.CateService.GetAll()

	// 浏览次数累加
	services.NovelService.UpViews(novel.Id)

	// 轮播广告图
	args := map[string]interface{}{
		"zone": int(services.ZONENOV),
	}
	slices := services.BannerService.GetAll(args)
	this.Data["BannerSlices"] = slices

	// 获取推荐的搜索关键字
	this.Data["RecKw"] = services.SearchService.GetRes(5)

	this.View("book/index.tpl")
}

// 查看小说章节内容
func (this *BookController) Detail() {
	id, _ := this.GetUint64("id")
	novId, _ := this.GetUint32("novid")
	if id < 1 || novId < 1 {
		this.Msg("参数错误，无法访问")
	}

	// 获取小说章节信息
	chap := services.ChapterService.Get(id, novId)
	if chap == nil {
		this.Msg("该章节不存在或者已被删除")
	}

	// 浏览次数增加
	services.ChapterService.UpdateViews(chap)

	// 获取小说详情
	nov := services.NovelService.Get(chap.NovId)
	if nov == nil {
		this.Msg("该小说不存在或者已被删除")
	}

	this.Data["Nov"] = nov
	this.Data["Chap"] = chap
	this.Data["Pre"] = services.ChapterService.GetPre(chap.NovId, chap.ChapterNo)
	this.Data["Next"] = services.ChapterService.GetNext(chap.NovId, chap.ChapterNo)
	this.Data["Title"] = chap.Title + " - " + nov.Name

	// 获取章节列表
	chaps, _ := services.ChapterService.GetNovChaps(nov.Id, 10000, 0, "asc", false)
	this.Data["Chaps"] = chaps

	this.Layout = ""
	this.View("book/detail.tpl")
}

// ajax加载小说章节
func (this *BookController) AjaxChap() {
	id, _ := this.GetUint64("id")
	novId, _ := this.GetUint32("novid")
	if id < 1 || novId < 1 {
		this.Msg("参数错误，无法访问")
	}

	// 获取小说章节信息
	chap := services.ChapterService.Get(id, novId)
	if chap == nil {
		this.Msg("该章节不存在或者已被删除")
	}

	tmp := map[string]interface{}{
		"id":    chap.Id,
		"title": chap.Title,
		"desc":  chap.Desc,
	}

	// 获取下一章节ID
	next := services.ChapterService.GetNext(chap.NovId, chap.ChapterNo)
	nextId := uint64(0)
	if next != nil {
		nextId = next.Id
	}

	res := map[string]interface{}{
		"chap":    tmp,
		"next_id": nextId,
	}
	this.OutJson(0, "", res)
}

// ajax获取小说列表
func (this *BookController) AjaxRank() {
	var novels []*models.Novel
	switch this.GetString("rank") {
	case "1":
		novels = services.NovelService.GetTodayRecs(10, 0)
	case "2":
		novels = services.NovelService.GetHots(10, 0)
	case "3": // 新书榜
		novels = services.NovelService.GetNews(10, 0)
	case "4": // 原创榜
		novels = services.NovelService.GetOriginals(10, 0)
	case "5": // VIP推荐榜
		novels = services.NovelService.GetVipRewards(10, 0)
	case "6": // VIP更新
		novels = services.NovelService.GetVipUps(10, 0)
	case "cate":
		cateId, _ := this.GetInt("cate_id")
		// 获取指定分类小说列表
		search := map[string]interface{}{
			"cate_id": cateId,
			"count":   false,
		}
		novels, _ = services.NovelService.GetList(6, 0, search)
	default:
		// 获取小说列表
		search := map[string]interface{}{
			"count": false,
		}
		novels, _ = services.NovelService.GetList(10, 0, search)
	}

	var list []map[string]interface{}

	for _, nov := range novels {
		tmp := make(map[string]interface{})

		tmp["id"] = nov.Id
		tmp["name"] = nov.Name
		tmp["author"] = nov.Author
		tmp["cate_name"] = nov.CateName
		tmp["cover"] = nov.Cover
		tmp["desc"] = beego.HTML2str(nov.Desc)

		list = append(list, tmp)
	}

	res := map[string]interface{}{
		"list": list,
	}
	this.OutJson(0, "", res)
}

// Download 下载整本小说为 TXT 文件
//
// 路由：GET /book/{id}/download.html（兼容 /book/download?id={id}）
// 具体导出逻辑在 controllers.BaseController.DownloadNovel 中，
// 由 PC 与移动端共用。
func (this *BookController) Download() {
	id, _ := this.GetUint32("id")
	this.DownloadNovel(id)
}
