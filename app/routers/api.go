package routers

import (
	"github.com/astaxie/beego"

	"github.com/vckai/novel/app/controllers/api"
)

/*
只读 API 路由

独立命名空间 /api，与前台页面路由完全分开，互不干扰。

设计考虑：这里用显式路由而非 AutoRouter —— API 路径是对外契约，
显式声明可以避免控制器方法改名导致接口地址悄然变化。
*/
func apiRouters() {
	ns := beego.NewNamespace("/api",

		// 首页聚合
		beego.NSRouter("/home", &api.NovelController{}, "GET:Home;OPTIONS:Home"),

		// 分类
		beego.NSRouter("/cates", &api.NovelController{}, "GET:Cates;OPTIONS:Cates"),

		// 书籍列表与搜索
		beego.NSRouter("/books", &api.NovelController{}, "GET:Books;OPTIONS:Books"),
		beego.NSRouter("/search", &api.NovelController{}, "GET:Search;OPTIONS:Search"),

		// 排行榜
		beego.NSRouter("/rank", &api.NovelController{}, "GET:Rank;OPTIONS:Rank"),

		// 书籍详情
		beego.NSRouter("/book/:id([0-9]+)", &api.NovelController{}, "GET:Detail;OPTIONS:Detail"),

		// 章节目录
		beego.NSRouter("/book/:id([0-9]+)/chapters", &api.NovelController{}, "GET:Chapters;OPTIONS:Chapters"),

		// 章节正文
		beego.NSRouter("/chapter/:id([0-9]+)", &api.NovelController{}, "GET:Chapter;OPTIONS:Chapter"),
	)

	beego.AddNamespace(ns)
}
