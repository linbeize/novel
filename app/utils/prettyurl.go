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

package utils

import (
	"strings"
)

// 伪静态 URL 生成与改写（PC 与移动端）
//
// 目标：把
//
//	PC  : /book/index?id=1          -> /book/1.html
//	      /book/detail?novid=1&id=2 -> /book/1/2.html
//	手机: /m/book/index?id=1          -> /m/book/1.html
//	      /m/book/detail?novid=1&id=2 -> /m/book/1/2.html
//
// 放置在此包（而非 routers）的原因：controllers 需要调用本函数来改写 URL，
// 但 routers 已经导入了 controllers，若把函数放 routers 会造成循环依赖。
// utils 被两者共同依赖，是合适的位置。
//
// 说明：beego 的 URLFor 会优先反查 AutoRouter 生成的旧式地址
// （实测返回 /book/index?id=1），因此无法通过路由反查得到伪静态地址，
// 这里改为按「控制器 + 方法」直接拼装。
//
// 注意：本站移动端并非子域而是 /m 前缀（MobileURL 为空时不剥离前缀），
// 因此改写时保留 /m 前缀，只替换其后的部分。

// 从 URL 查询串中取出指定参数（不依赖 net/url，避免解码等副作用）
func queryValue(rawurl, key string) string {
	i := strings.Index(rawurl, "?")
	if i < 0 {
		return ""
	}

	for _, kv := range strings.Split(rawurl[i+1:], "&") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1]
		}
	}

	return ""
}

// 按前缀与端点信息拼装伪静态地址
//
// prefix 为站内模块前缀（PC 为空串，移动端为 "/m"）
func buildPrettyURL(prefix, endpoint, url string) string {
	switch endpoint {
	case "Index":
		// 小说详情：?id=N
		if id := queryValue(url, "id"); id != "" {
			return prefix + "/book/" + id + ".html"
		}
	case "Detail":
		// 章节阅读：?novid=N&id=M
		novid := queryValue(url, "novid")
		id := queryValue(url, "id")
		if novid != "" && id != "" {
			return prefix + "/book/" + novid + "/" + id + ".html"
		}
	case "Download":
		// 整本下载：?id=N
		if id := queryValue(url, "id"); id != "" {
			return prefix + "/book/" + id + "/download.html"
		}
	}

	return url
}

// 分类页伪静态地址
//
// 形式（PC）：
//
//	/cate/1.html                          第 1 页、无筛选
//	/cate/1/p2.html                       第 2 页、无筛选
//	/cate/1/p2/1_0_0_1.html               第 2 页 + 筛选(status_textnum_uptime_ot)
//
// 移动端加 /m 前缀。之所以把 4 个筛选值用下划线连成一段，
// 是为了避免路径层级过深，同时让「无筛选」也能用固定段数表达
// （便于用一条路由规则覆盖所有组合）。
const (
	prettyCateDefaultOt = 1 // 默认排序：人气
)

// 生成小说详情页「目录分页」的伪静态地址
//
// 形式：
//
//	/book/805.html          第 1 页
//	/book/805/p3.html       第 3 页
//
// 之所以不放在 /book/{id}.html 后面加查询串（?p=3），是为了与其他
// 伪静态地址风格统一。
// 注意第 1 页保持不带分页段的短地址，便于作为规范链接。
func PrettyBookCatalogURL(prefix string, novId, page int) string {
	if novId < 1 {
		return prefix + "/book/"
	}

	base := prefix + "/book/" + itoa(novId)

	if page <= 1 {
		return base + ".html"
	}

	return base + "/p" + itoa(page) + ".html"
}

// 生成分类页地址
// prefix: "" 或 "/m"；cateId: 分类；page: 页码；其余为筛选值
func PrettyCateURL(prefix string, cateId, page, status, textNum, upTime, ot int) string {
	if cateId < 1 {
		return prefix + "/cate/"
	}

	base := prefix + "/cate/" + itoa(cateId)

	// 第 1 页且无任何筛选 -> 最简形式
	if page <= 1 && status == 0 && textNum == 0 && upTime == 0 && ot == prettyCateDefaultOt {
		return base + ".html"
	}

	// 有筛选时按固定 5 段输出；筛选值全为默认时省略筛选段
	noFilter := status == 0 && textNum == 0 && upTime == 0 && ot == prettyCateDefaultOt
	if noFilter {
		if page <= 1 {
			return base + ".html"
		}
		return base + "/p" + itoa(page) + ".html"
	}

	p := page
	if p < 1 {
		p = 1
	}

	return base + "/p" + itoa(p) + "/" +
		itoa(status) + "_" + itoa(textNum) + "_" + itoa(upTime) + "_" + itoa(ot) + ".html"
}

// 轻量整型转字符串（避免为此引入 strconv 之外的依赖）
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	neg := i < 0
	if neg {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}

// 分类页路由前缀（PC/移动端）
func CatePrefix(mobile bool) string {
	if mobile {
		return "/m"
	}

	return ""
}

// 将旧式地址改写为伪静态地址
//
// 覆盖 PC 与移动端的：小说详情、章节阅读、分类页。
// 其它控制器（搜索、排行、Ajax 接口等）保持原样。
func PrettyURL(endpoint, url string) string {
	switch endpoint {
	case "home.BookController.Index":
		return buildPrettyURL("", "Index", url)
	case "home.BookController.Detail":
		return buildPrettyURL("", "Detail", url)
	case "home.BookController.Download":
		return buildPrettyURL("", "Download", url)
	case "m.BookController.Index":
		return buildPrettyURL("/m", "Index", url)
	case "m.BookController.Detail":
		return buildPrettyURL("/m", "Detail", url)
	case "m.BookController.Download":
		return buildPrettyURL("/m", "Download", url)

	case "home.HomeController.Cate":
		return buildPrettyCateFromQuery("", url)
	case "m.BookController.List":
		return buildPrettyCateFromQuery("/m", url)
	}

	return url
}

// 从旧式分类地址的 query 中解析参数并生成伪静态地址
//
// 旧式：/home/cate?id=1&p=2&status=1&text_num=0&uptime=0&ot=1
// 新式：/cate/1/p2/1_0_0_1.html
func buildPrettyCateFromQuery(prefix, url string) string {
	cateId := atoiDefault(queryValue(url, "id"), 0)
	// 移动端使用 cate_id
	if cateId == 0 {
		cateId = atoiDefault(queryValue(url, "cate_id"), 0)
	}

	// 未指定分类（如导航「分类」入口）时保持原地址：
	// 伪静态路径必须以分类 id 打头，否则会生成 /cate/ 这种无效地址。
	if cateId < 1 {
		return url
	}

	page := atoiDefault(queryValue(url, "p"), 1)
	status := atoiDefault(queryValue(url, "status"), 0)
	textNum := atoiDefault(queryValue(url, "text_num"), 0)
	upTime := atoiDefault(queryValue(url, "uptime"), 0)
	ot := atoiDefault(queryValue(url, "ot"), prettyCateDefaultOt)

	return PrettyCateURL(prefix, cateId, page, status, textNum, upTime, ot)
}

// 字符串转 int，失败返回默认值
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}

	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}

	return n
}
