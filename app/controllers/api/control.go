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
	"net/http"

	"github.com/vckai/novel/app/controllers"
	"github.com/vckai/novel/app/services"
)

/*
只读 API 基础控制器
==================

供 App 对接使用。几点设计说明：

1. 继承最底层的 controllers.BaseController，而不是 home.BaseController
   —— 后者会做移动端 UA 检测并 302 跳转到 /m，App 请求多带移动端 UA，
   会被重定向到 HTML 页面。API 必须始终返回 JSON，不能有跳转。

2. 统一的响应结构，与站内既有 Ajax 接口保持一致，降低对接成本：

       {"ret":0,"msg":"","data":...}

   ret 为 0 表示成功，非 0 表示出错（参考下方错误码）。

3. 移动端/App 属跨域场景，统一放开 CORS 并处理预检请求。

4. 只读：不提供写接口，因此无需鉴权，也不涉及用户数据。
*/

// 错误码
//
// 与站内既有接口保持一致的语义：0 成功、1 参数或业务错误。
// 1001 用于「资源不存在」，便于客户端区分「请求写错了」与「内容没了」。
const (
	CodeOK       = 0
	CodeParamErr = 1
	CodeNotFound = 1001
)

// 分页默认值
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type BaseController struct {
	controllers.BaseController
}

// 小说相关只读接口
//
// 首页聚合、分类、书籍列表/搜索、详情、目录、正文等，
// 逻辑相近故放在同一个控制器里。
type NovelController struct {
	BaseController
}

// 统一的前置处理
//
// 放开跨域并处理 OPTIONS 预检；同时声明返回 JSON，
// 避免浏览器按 HTML 解析。
func (this *BaseController) Prepare() {
	this.BaseController.Prepare()

	// CORS：只读接口，允许任意来源，便于 App / 小程序 / 网页调用
	this.Ctx.Output.Header("Access-Control-Allow-Origin", "*")
	this.Ctx.Output.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	this.Ctx.Output.Header("Access-Control-Allow-Headers", "Content-Type, X-Requested-With")

	// 预检请求直接返回，不进入业务逻辑
	if this.Ctx.Input.Method() == http.MethodOptions {
		this.Ctx.Output.SetStatus(http.StatusNoContent)
		this.StopRun()
	}
}

// 成功响应
func (this *BaseController) ok(data interface{}) {
	this.Data["json"] = map[string]interface{}{
		"ret":  CodeOK,
		"msg":  "",
		"data": data,
	}
	this.ServeJSON()
	this.StopRun()
}

// 错误响应
func (this *BaseController) fail(code int, msg string) {
	this.Data["json"] = map[string]interface{}{
		"ret":  code,
		"msg":  msg,
		"data": nil,
	}
	this.ServeJSON()
	this.StopRun()
}

// 取分页参数，做边界限制，避免被要求返回巨量数据
func (this *BaseController) pageParams() (page, size, offset int) {
	page, _ = this.GetInt("p", 1)
	if page < 1 {
		page = 1
	}

	size, _ = this.GetInt("size", defaultPageSize)
	if size < 1 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	offset = (page - 1) * size
	return
}

// 把相对路径的封面地址补成绝对地址
//
// 库中 cover 存的是 /public/up/... 这类相对路径，App 侧无法直接加载。
// 站点根地址取自后台配置 WebURL；未配置时退回请求的 Host。
// 已经是 http(s) 开头的地址原样返回（部分来源站直接存了外链）。
func (this *BaseController) absURL(path string) string {
	if path == "" {
		return ""
	}

	if len(path) > 4 && (path[:4] == "http" || path[:2] == "//") {
		return path
	}

	base := services.ConfigService.String("WebURL")
	if base == "" {
		scheme := "http"
		if this.Ctx.Input.IsSecure() {
			scheme = "https"
		}
		base = scheme + "://" + this.Ctx.Request.Host
	}

	// 去掉 base 末尾的 / 与 path 开头的 /，避免拼出双斜杠
	for len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	if path[0] != '/' {
		path = "/" + path
	}

	return base + path
}
