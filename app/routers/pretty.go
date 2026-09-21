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

package routers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"

	"github.com/vckai/novel/app/controllers/home"
	"github.com/vckai/novel/app/controllers/m"
)

// 注册前台伪静态路由（PC 与移动端）
//
// 目标地址：
//
//	/book/1.html        小说详情（原 /book/index?id=1）
//	/book/1/2.html      章节阅读（原 /book/detail?novid=1&id=2）
//	/m/book/1.html      移动端小说详情（原 /m/book/index?id=1）
//	/m/book/1/2.html    移动端章节阅读（原 /m/book/detail?novid=1&id=2）
//
// 三个实测确认的要点：
//
//  1. 参数键名带冒号：beego 把路径参数存为 ":id"，实测 Param(":id")="42"
//     而 Param("id")=""；控制器里用的是 GetUint32("id")。
//     因此这里用过滤器把 ":id" 复制成 "id"，控制器代码无需改动。
//
//  2. .html 会被 beego 当作扩展名自动剥离（allowSuffixExt），
//     所以正则中不要吞掉 .html，写 :id([0-9]+).html 即可。
//
//  3. 必须保留原 AutoRouter：旧链接 /book/index?id=1 仍需可访问，
//     否则已收录地址与站内历史链接会 404（向后兼容）。
func prettyRouters() {
	// PC 前台：小说详情
	beego.Router("/book/:id([0-9]+).html", &home.BookController{}, "GET:Index")
	beego.Router("/book/:novid([0-9]+)/:id([0-9]+).html", &home.BookController{}, "GET:Detail")

	// PC 前台：整本小说下载
	//   /book/1/download.html
	// 放在详情路由之后注册，避免与 :id 规则互相遮蔽。
	beego.Router("/book/:id([0-9]+)/download.html", &home.BookController{}, "GET:Download")

	// PC 前台：详情页「目录分页」
	//   /book/1.html       第 1 页
	//   /book/1/p3.html    第 3 页
	// 同样注册在详情路由之后，两个规则的第二段分别是 p3.html 与 download.html，
	// 与 :id([0-9]+) 形态不同，不会互相遮蔽。
	beego.Router("/book/:id([0-9]+)/p:page([0-9]+).html", &home.BookController{}, "GET:Index")

	// PC 前台：分类页
	//   /cate/1.html                          第 1 页、无筛选
	//   /cate/1/p2.html                       第 2 页、无筛选
	//   /cate/1/p2/1_0_0_1.html               页码 + 4 个筛选值(status_textnum_uptime_ot)
	// 末段用下划线连接，可用一条规则覆盖全部筛选组合
	beego.Router("/cate/:id([0-9]+).html", &home.HomeController{}, "GET:Cate")
	beego.Router("/cate/:id([0-9]+)/p:page([0-9]+).html", &home.HomeController{}, "GET:Cate")
	beego.Router("/cate/:id([0-9]+)/p:page([0-9]+)/:status([0-9]+)_:textnum([0-9]+)_:uptime([0-9]+)_:ot([0-9]+).html",
		&home.HomeController{}, "GET:Cate")

	// 移动端（命名空间 /m）
	mPretty := beego.NewNamespace("/m",
		beego.NSRouter("/book/:id([0-9]+).html", &m.BookController{}, "GET:Index"),
		beego.NSRouter("/book/:novid([0-9]+)/:id([0-9]+).html", &m.BookController{}, "GET:Detail"),
		beego.NSRouter("/book/:id([0-9]+)/download.html", &m.BookController{}, "GET:Download"),

		beego.NSRouter("/cate/:id([0-9]+).html", &m.BookController{}, "GET:List"),
		beego.NSRouter("/cate/:id([0-9]+)/p:page([0-9]+).html", &m.BookController{}, "GET:List"),
		beego.NSRouter("/cate/:id([0-9]+)/p:page([0-9]+)/:status([0-9]+)_:textnum([0-9]+)_:uptime([0-9]+)_:ot([0-9]+).html",
			&m.BookController{}, "GET:List"),
	)
	beego.AddNamespace(mPretty)

	// 路径参数别名：把带冒号的路径参数复制为控制器使用的参数名，
	// 使控制器里原有的 GetUint32("id") / GetInt("cate_id") 等无需改动即可工作。
	//
	// 注意：PC 分类页控制器读的是 "id"，移动端分类页读的是 "cate_id"，
	// 而伪静态路径里统一用 ":id"，因此这里同时写上两个别名。
	beego.InsertFilter("/*", beego.BeforeExec, func(ctx *context.Context) {
		aliases := map[string][]string{
			":id":      {"id", "cate_id"},
			":novid":   {"novid"},
			":page":    {"p"},
			":status":  {"status"},
			":textnum": {"text_num"},
			":uptime":  {"uptime"},
			":ot":      {"ot"},
		}

		for src, dsts := range aliases {
			v := ctx.Input.Param(src)
			if v == "" {
				continue
			}
			for _, d := range dsts {
				ctx.Input.SetParam(d, v)
			}
		}
	})
}
