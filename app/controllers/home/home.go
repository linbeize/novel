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
	"math"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services"
	"github.com/vckai/novel/app/utils"
)

type HomeController struct {
	BaseController
}

// 首页
func (this *HomeController) Index() {
	// 获取小编推荐
	recs := services.NovelService.GetRecs(20, 0)
	var recsData [][]*models.Novel
	// 分页显示
	if recs != nil && len(recs) > 0 {
		max := 4
		recsLen := len(recs)
		for i := 0; i < recsLen; i += max {
			index := int(math.Min(float64(i+max), float64(recsLen)))
			recsData = append(recsData, recs[i:index])
		}
	}
	this.Data["NovRecsNum"] = len(recsData)
	this.Data["NovRecs"] = recsData

	// 获取最新更新
	this.Data["NovNews"] = services.NovelService.GetNewUps(12, 0)

	// 获取阅读排行榜
	novRanks := services.NovelService.GetRanks(9, 0)
	this.Data["NovRank"] = nil
	this.Data["NovRanks"] = nil
	if len(novRanks) > 0 {
		this.Data["NovRank"] = novRanks[0:1][0]
		if len(novRanks) > 1 {
			this.Data["NovRanks"] = novRanks[1:]
		}
	}

	// 获取签约新书推荐
	this.Data["NovSignNewBooks"] = services.NovelService.GetSignNewBooks(3, 0)

	// 获取收藏
	this.Data["NovCollects"] = services.NovelService.GetCollects(3, 0)

	// 获取精品推荐
	this.Data["NovVipRecs"] = services.NovelService.GetVipRecs(6, 0)

	// 获取菜单分类
	cates := services.CateService.GetAll()
	this.Data["Cates"] = cates

	// 获取推荐分类
	var cateRecs []*models.Cate
	for _, v := range cates {
		if v.IsHome == 1 {
			cateRecs = append(cateRecs, v)
		}
	}
	this.Data["CateRecsNum"] = len(cateRecs) - 1
	this.Data["CateRecs"] = cateRecs

	// 轮播广告图
	args := map[string]interface{}{
		"zone": int(services.ZONEINDEXSLICE),
	}
	slices := services.BannerService.GetAll(args)
	this.Data["BannerSlices"] = slices
	this.Data["BannerSlicesCount"] = len(slices)

	// 中间两个小广告图
	args = map[string]interface{}{
		"zone": int(services.ZONEINDEXBANNER),
	}
	this.Data["BannerSmalls"] = services.BannerService.GetAll(args)

	// 获取推荐的搜索关键字
	this.Data["RecKw"] = services.SearchService.GetRes(5)

	this.Data["Title"] = "首页"
	this.View("home/index.tpl")
}

// 全部作品
func (this *HomeController) Cate() {
	size := 10
	// 页码规范化：伪静态或 query 都可能传入 0、负数或非数字，
	// 直接参与 offset 计算会产生负偏移，导致返回异常结果。
	p, _ := this.GetInt("p", 1)
	if p < 1 {
		p = 1
	}
	offset := (p - 1) * size

	// 分类ID
	cid, _ := this.GetInt("id", 0)

	// 状态
	status, _ := this.GetInt("status", 0)

	// 字数
	textNum, _ := this.GetInt("text_num", 0)

	// 更新时间
	upTime, _ := this.GetInt("uptime", 0)

	// 排序方式
	ot, _ := this.GetInt("ot", 1)

	search := map[string]interface{}{
		"count":    true,
		"cate_id":  cid,
		"status":   status,
		"text_num": textNum,
		"uptime":   upTime,
		"ot":       ot,
	}

	// 取当页数据（同时得到总数，一次查询完成）
	novs, count := services.NovelService.GetList(size, offset, search)

	// 页码越界时收敛到最后一页并重取。
	// 注意不能先单独查总数：GetAll 在 count>0 时仍会执行数据查询，
	// 若 Limit 传 0 则会把全部匹配行取出，反而不如直接取当页。
	maxPage := 1
	if count > 0 {
		maxPage = int((count + int64(size) - 1) / int64(size))
	}
	if p > maxPage {
		p = maxPage
		offset = (p - 1) * size
		novs, _ = services.NovelService.GetList(size, offset, search)
	}

	// 设置分页
	// 注入伪静态地址生成器，使分页链接输出 /cate/1/p2.html 形式。
	// 同时用 SetPage 显式告知当前页码：伪静态把页码放在路径中，
	// 分页器默认只读 query 的 p，不设置会导致页码高亮与前后页判断错误。
	pager := this.SetPaginator(size, count)
	pager.SetPage(p)
	cateId, statusV, textNumV, upTimeV, otV := cid, status, textNum, upTime, ot
	pager.URLBuilder = func(page int) string {
		return utils.PrettyCateURL("", cateId, page, statusV, textNumV, upTimeV, otV)
	}

	this.Data["NovCount"] = count
	this.Data["Novs"] = novs

	// 获取推荐的搜索关键字
	this.Data["RecKw"] = services.SearchService.GetRes(5)

	// 获取菜单分类
	this.Data["Cates"] = services.CateService.GetAll()

	this.Data["CateId"] = uint32(cid)
	this.Data["StatusList"] = models.NovelModel.StatusList()
	this.Data["Status"] = uint8(status)
	search["p"] = p
	this.Data["Search"] = search
	this.Data["Title"] = "全部作品"

	this.View("home/cate.tpl")
}

// 搜索结果
func (this *HomeController) Search() {
	size := 10
	q := this.GetString("kw")
	ot, _ := this.GetInt("ot", 1)

	search := map[string]interface{}{
		"q":     q,
		"count": true,
		"ot":    ot,
	}
	p, _ := this.GetInt("p", 1)
	offset := (p - 1) * size

	// 搜索小说
	novs, count := services.NovelService.GetList(size, offset, search)

	log := &models.SearchLog{
		Kw: q,
		Ip: this.Ctx.Input.IP(),
	}

	if len(novs) > 0 {
		log.IsResult = 1
	}
	services.SearchService.InsertOrIncrement(q, log)

	// 设置分页
	this.SetPaginator(size, count)

	// 获取排行榜
	this.Data["NovRanks"] = services.NovelService.GetRanks(6, 0)

	// 获取菜单分类
	this.Data["Cates"] = services.CateService.GetAll()

	// 获取推荐的搜索关键字
	this.Data["RecKw"] = services.SearchService.GetRes(5)

	this.Data["Search"] = search
	this.Data["NovCount"] = count
	this.Data["Novs"] = novs
	this.Data["Title"] = q + "搜索结果"

	this.View("home/search.tpl")
}
func (this *HomeController) History() {
	
	this.Data["Title"] = "历史记录"
	this.Data["Cates"] = services.CateService.GetAll()
	this.View("home/history.tpl")
	
}
