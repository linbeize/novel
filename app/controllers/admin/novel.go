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

package admin

import (
	"strings"
	"time"

	"github.com/vckai/novel/app/controllers"
	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services"
	"github.com/vckai/novel/app/services/snatchs"
	"github.com/vckai/novel/app/utils/log"
)

type NovelController struct {
	BaseController
}

// 小说列表
func (this *NovelController) Index() {
	kw := this.GetString("kw")
	size := 10
	p, _ := this.GetInt("p", 1)

	args := models.ArgsNovelList{}
	args.Count = true
	args.Limit = size
	args.Offset = (p - 1) * size
	args.Keyword = kw

	novels, count := services.NovelService.GetAll(args)

	this.Data["Search"] = map[string]interface{}{
		"kw":    kw,
		"page":  p,
		"limit": size,
	}
	this.Data["List"] = novels
	this.Data["Count"] = count
	this.View("novel/index.tpl")
}

// 添加小说页面
func (this *NovelController) Add() {
	// post 数据提交
	if this.IsAjax() {
		this.save()
		return
	}

	this.Data["Novel"] = models.NewNovel()
	this.Data["IsUpdate"] = false
	this.Data["Cates"] = services.CateService.GetAll()
	this.Data["PostUrl"] = controllers.URLFor("admin.NovelController.Add")
	this.View("novel/add.tpl")
}

// 运行爬虫
func (this *NovelController) Crawler() {
	// post 数据提交
	if this.IsAjax() {
		source := this.GetString("source")

		crawler, err := services.CrawlerService.Init(source)
		if err != nil {
			this.OutJson(1001, "爬虫运行失败："+err.Error())
		}

		go crawler.Run()

		this.OutJson(0, "爬虫运行成功")

		return
	}

	this.Data["Snatchs"] = services.SnatchRuleService.GetSnatchs()
	this.View("novel/crawler.tpl")
}

// 添加采集单本小说
func (this *NovelController) AddSnatch() {
	// post 数据提交
	if this.IsAjax() {
		url := this.GetString("url")
		err := services.SnatchService.InitNovel(url)
		if err != nil {
			this.OutJson(1001, "添加小说采集失败："+err.Error())
		}

		this.OutJson(0, "添加小说成功")
		return
	}

	this.Data["Cates"] = services.CateService.GetAll()
	this.View("novel/add_snatch.tpl")
}

// 手动执行采集任务
func (this *NovelController) Snatch() {
	id, _ := this.GetUint32("id")
	name := this.GetString("name")
	if id < 1 {
		this.Msg("参数错误，无法访问")
	}

	// 执行小说采集任务
	err := services.SnatchService.SnatchNovel(id)
	if err != nil {
		this.OutJson(1001, "小说采集任务手动执行失败："+err.Error())
	}

	// 添加操作日记
	this.AddLog(3002, "手动执行采集任务", name, id)

	this.OutJson(0, "执行成功！")

}

// 从采集站中搜索小说
func (this *NovelController) FindSnatchs() {
	kw := this.GetString("kw")
	if len(kw) == 0 {
		this.Msg("请输入小说/作者名称")
	}

	t0 := time.Now()

	novs := services.SnatchService.FindNovels(kw)

	// 按站点分组，便于页面上分站对比
	type siteGroup struct {
		Site  string
		Count int
		Items []*snatchs.SnatchInfo
	}

	order := make([]string, 0)
	byName := map[string]*siteGroup{}

	for _, n := range novs {
		name := n.SiteName
		if name == "" {
			name = n.Source
		}

		g, ok := byName[name]
		if !ok {
			g = &siteGroup{Site: name}
			byName[name] = g
			order = append(order, name)
		}

		g.Items = append(g.Items, n)
		g.Count++
	}

	groups := make([]*siteGroup, 0, len(order))
	for _, name := range order {
		groups = append(groups, byName[name])
	}

	// 已在站内的书名（用于标注「已收录」，避免重复采集）
	existing := map[string]bool{}
	for _, n := range novs {
		if n.Nov == nil {
			continue
		}
		if got := services.NovelService.GetByName(n.Nov.Name); got != nil && got.Id > 0 {
			existing[n.Nov.Name] = true
		}
	}

	this.Data["Cates"] = services.CateService.GetAll()
	this.Data["Groups"] = groups
	this.Data["NovsCount"] = len(novs)
	this.Data["SiteCount"] = len(groups)
	this.Data["Existing"] = existing
	this.Data["Kw"] = kw
	this.Data["Elapsed"] = time.Since(t0).Round(time.Millisecond).String()

	this.View("novel/find_snatch.tpl")
}

// 获取采集站点列表
func (this *NovelController) Links() {
	novId, _ := this.GetUint32("novid")
	if novId < 1 {
		this.Msg("参数错误，无法访问")
	}

	novLinks := services.NovelService.GetLinks(novId)
	this.Data["NovLinks"] = novLinks
	this.Data["NovLinksCount"] = len(novLinks)
	this.View("novel/links.tpl")
}

// 编辑小说页面
func (this *NovelController) Edit() {
	// post 数据提交
	if this.IsAjax() {
		this.save()
		return
	}

	id, _ := this.GetUint32("id")
	if id < 1 {
		this.Msg("参数错误，无法访问")
	}

	novel := services.NovelService.Get(id)
	if novel == nil {
		this.Msg("该小说不存在或者已被删除")
	}

	this.Data["Novel"] = novel
	this.Data["IsUpdate"] = true
	this.Data["Cates"] = services.CateService.GetAll()
	this.Data["PostUrl"] = controllers.URLFor("admin.NovelController.Edit")
	this.View("novel/add.tpl")
}

// 删除小说
func (this *NovelController) Delete() {
	id, _ := this.GetUint32("id")
	name := this.GetString("name")
	if id < 1 {
		this.Msg("参数错误，无法访问")
	}

	err := services.NovelService.Delete(id)
	if err != nil {
		this.OutJson(1001, "删除小说失败")
	}

	// 添加操作日记
	this.AddLog(3002, "删除", name, id)

	this.OutJson(0, "已删除！")
}

// 批量删除小说
func (this *NovelController) DeleteBatch() {
	ids := this.GetStrings("ids[]")
	if len(ids) == 0 {
		this.OutJson(1001, "参数错误，无法访问")
	}

	err := services.NovelService.DeleteBatch(ids)
	if err != nil {
		this.OutJson(1002, "批量删除小说失败")
	}

	// 添加操作日记
	this.AddLog(3005, strings.Join(ids, ","))

	this.OutJson(0, "已删除！")
}

// 清空小说章节
func (this *NovelController) DelChapters() {
	id, _ := this.GetUint32("id")
	name := this.GetString("name")
	if id < 1 {
		this.Msg("参数错误，无法访问")
	}

	err := services.NovelService.DelChapters(id)
	if err != nil {
		this.OutJson(1001, "清空小说章节失败"+err.Error())
	}

	// 添加操作日记
	this.AddLog(3002, "清空章节", name, id)

	this.OutJson(0, "已清空！")
}

// 删除小说采集站点
func (this *NovelController) DelLink() {
	id, _ := this.GetUint32("id")
	novId, _ := this.GetUint32("nov_id")
	source := this.GetString("source")
	if id < 1 {
		this.Msg("参数错误，无法访问")
	}

	err := services.NovelService.DelLink(id)
	if err != nil {
		this.OutJson(1001, "删除小说站点失败")
	}

	// 添加操作日记
	this.AddLog(3003, "删除", novId, source, id)

	this.OutJson(0, "已删除！")
}

// 保存数据
// 提供修改/新增处理
func (this *NovelController) save() {
	id, _ := this.GetUint32("novel_id")

	mtitle := "添加"
	novel := models.NewNovel()
	// 编辑
	if id > 0 {
		novel = services.NovelService.Get(id)
		if novel == nil {
			this.OutJson(1001, "该小说不存在或者已被删除")
		}

		mtitle = "修改"
	}

	// 入库参数
	novel.Name = this.GetString("name")
	novel.Desc = this.GetString("desc")
	novel.Cover = this.GetString("cover")
	novel.Author = this.GetString("author")
	novel.CateId, _ = this.GetUint32("cate_id")
	novel.IsOriginal, _ = this.GetUint8("is_original")
	novel.IsHot, _ = this.GetUint8("is_hot")
	novel.IsRec, _ = this.GetUint8("is_rec")
	novel.IsVipRec, _ = this.GetUint8("is_vip_rec")
	novel.IsTodayRec, _ = this.GetUint8("is_today_rec")
	novel.IsSignNewBook, _ = this.GetUint8("is_sign_new_book")
	novel.IsCollect, _ = this.GetUint8("is_collect")
	novel.IsVipReward, _ = this.GetUint8("is_vip_reward")
	novel.IsVipUp, _ = this.GetUint8("is_vip_up")
	novel.Status, _ = this.GetUint8("status")

	err := services.NovelService.Save(novel)
	if err != nil {
		log.Error(mtitle, "小说失败：", err.Error())
		this.OutJson(1002, mtitle+"小说失败："+err.Error())
	}

	// 添加操作日记
	this.AddLog(3002, mtitle, novel.Name, novel.Id)

	this.OutJson(0, mtitle+"成功")
}
