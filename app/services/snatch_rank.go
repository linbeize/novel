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

/*
起点排行榜采集
==============

历史问题
--------
原实现请求 https://www.qidian.com/rank/{hotsales,recom,collect,...}?style=1，
但起点现已对该路径启用 JS 挑战反爬：任何请求都返回 HTTP 202 加一个仅含
probe.js 的空壳页面（实测约 209 字节），页面里没有任何榜单数据。
同时旧的榜单路径也已调整，原 8 个地址中多数会返回 404。

修复方案
--------
改用移动端站点 https://m.qidian.com/rank/{name}/ ：

  - 该路径返回服务端渲染的完整 HTML（实测约 65KB、20 条书目），
    无需执行 JS，goquery 可直接解析；
  - 榜单数据在 <h2 class="_title_..."> 的文本中，是干净的书名
    （不带「最新章节在线阅读」这类后缀），因此可直接与库中 name 匹配
    —— 这一点很关键，UpRecBatch 是按书名精确匹配来标记推荐位的。

由于起点榜单名与库中 8 个推荐位字段并非一一对应，映射关系见 Qidian()
内的说明。移动端榜单每次返回 20 条，故每页取前 20 本。
*/

package services

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/axgle/mahonia"

	xhttp "github.com/vckai/novel/app/librarys/net/http"
	"github.com/vckai/novel/app/utils/log"
)

// 起点移动端榜单根地址
const qidianMobileRankBase = "https://m.qidian.com/rank"

// 移动端榜单的条目选择器
//
// 条目结构（实测）：
//
//	<div class="y-list__item" data-index="0">
//	  <a href="//m.qidian.com/book/1040765595/">
//	    <div class="_bookItemRight_...">
//	      <div class="_topTitle_..."><div class="_ranking_...">1</div>
//	        <h2 class="_title_..." title="畅销榜第1位">夜无疆</h2></div>
//	      <p class="_bookDesc_...">简介</p>
//	      <p class="_subTitle_...">辰东 · 玄幻 · 403.89万字</p>
//	    </div>
//	  </a>
//	</div>
//
// 注意类名带构建哈希（_title_1i1u2_355），故只能用前缀匹配。
const (
	qidianItemSelector  = "div.y-list__item"
	qidianTitleSelector = "h2[class^='_title_']"
)

type SnatchRank struct{}

func NewSnatchRank() *SnatchRank {
	return &SnatchRank{}
}

// 运行
func (this *SnatchRank) Run() {
	this.Qidian()
}

// 采集起点排行榜，并更新小说推荐位标记
//
// 映射说明（起点移动端榜单 -> 库中字段）：
//
//	hotsales  畅销榜 -> is_hot        （热度）
//	yuepiao   月票榜 -> is_vip_rec    （VIP 推荐）
//	readindex 阅读榜 -> is_collect    （收藏）
//	newfans   书友榜 -> is_vip_reward （打赏/粉丝）
//	rec       推荐榜 -> is_rec        （推荐）
//	update    更新榜 -> is_vip_up     （VIP 更新）
//	sign      签约榜 -> is_sign_new_book（新人签约）
//	newbook   新书榜 -> is_today_rec  （今日推荐）
//
// 同名旧榜单（recom、collect、vipcollect、signnewbook、vipreward、vipup）
// 在移动端已不存在，故按语义就近对应；若某榜单名变化导致取不到数据，
// 仅跳过该字段，不影响其余榜单。
func (this *SnatchRank) Qidian() {
	// 榜单名 -> 库中推荐位字段
	ranks := []struct {
		path  string
		field string
		desc  string
	}{
		{"hotsales", "is_hot", "畅销榜"},
		{"yuepiao", "is_vip_rec", "月票榜"},
		{"readindex", "is_collect", "阅读榜"},
		{"newfans", "is_vip_reward", "书友榜"},
		{"rec", "is_rec", "推荐榜"},
		{"update", "is_vip_up", "更新榜"},
		{"sign", "is_sign_new_book", "签约榜"},
		{"newbook", "is_today_rec", "新书榜"},
	}

	for _, r := range ranks {
		rawurl := qidianMobileRankBase + "/" + r.path + "/"

		books, err := this.getBooks(rawurl)
		if err != nil {
			log.Error("起点更新"+r.desc+"小说失败：", err)
			continue
		}

		if len(books) == 0 {
			// 榜单结构变化或受限时不应清空既有推荐位
			log.Warn("起点" + r.desc + "未取到任何书目，已跳过")
			continue
		}

		if err := NovelService.UpRecBatch(r.field, books); err != nil {
			log.Error("起点更新"+r.desc+"推荐位失败：", err)
		}
	}
}

// 获取榜单中的书名列表
func (this *SnatchRank) getBooks(rawurl string) ([]string, error) {
	books := []string{}

	doc, err := this.NewHtml(rawurl, "UTF-8")
	if err != nil {
		return books, err
	}

	seen := make(map[string]bool)

	doc.Find(qidianItemSelector).Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find(qidianTitleSelector).First().Text())
		if title == "" {
			return
		}

		// 去重：不同榜单或同一榜单内可能出现重复书目
		if seen[title] {
			return
		}
		seen[title] = true

		books = append(books, title)
	})

	return books, nil
}

// 网页请求
// 返回goquery格式内容
func (this *SnatchRank) NewHtml(rawurl, charset string) (*goquery.Document, error) {
	var res []byte
	var body io.Reader
	var err error

	c := xhttp.NewClient(
		&xhttp.ClientConfig{
			Timeout:   10 * time.Second,
			Dial:      10 * time.Second,
			KeepAlive: 60 * time.Second,
			ProxyURL:  ProxyService.Get(),
		})

	// 使用移动端 UA：起点的 PC 站点对普通请求会返回 JS 挑战页，
	// 而移动端站点直接返回服务端渲染的 HTML。
	res, _, err = c.Get(context.TODO(), rawurl, http.Header{
		"User-Agent": []string{qidianMobileUA},
	})
	if err != nil {
		return nil, err
	}

	body = bytes.NewReader(res)

	// 编码格式转码
	enc := mahonia.NewDecoder(charset)
	body = enc.NewReader(body)

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// 移动端 UA
const qidianMobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) " +
	"AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1"
