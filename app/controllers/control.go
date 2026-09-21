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

package controllers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/astaxie/beego"
	"github.com/beego/i18n"

	"github.com/vckai/novel/app/services"
	"github.com/vckai/novel/app/utils"
	"github.com/vckai/novel/app/utils/log"
)

// 控制器基类
type BaseController struct {
	beego.Controller
	i18n.Locale

	Module string

	// 请求性能统计
	perf *utils.Perf
}

// 初始化操作
func (this *BaseController) Prepare() {
	defLang := beego.AppConfig.String("lang::default")
	this.Lang = defLang

	// 开始统计本次请求耗时
	this.perf = utils.StartPerf()
}

// 请求结束：结束统计并释放资源
func (this *BaseController) Finish() {
	if this.perf != nil {
		this.perf.Stop()
	}
}

// 模板封装处理
/**
 * @api POST /user/:id get user
 */
func (this *BaseController) View(tpl string) {
	// 系统参数
	aOut := make(map[string]interface{})
	aOut["WebURL"] = services.ConfigService.String("WebURL")
	aOut["ViewUrl"] = services.ConfigService.String("ViewURL")
	aOut["Title"] = services.ConfigService.String("Title")
	aOut["SubTitle"] = services.ConfigService.String("SubTitle")
	aOut["Keyword"] = services.ConfigService.String("Keyword")
	aOut["Description"] = services.ConfigService.String("Description")
	aOut["Icp"] = services.ConfigService.String("Icp")
	aOut["Copyright"] = services.ConfigService.String("Copyright")
	aOut["StatisticsCode"] = services.ConfigService.String("StatisticsCode", "")
	aOut["Logo"] = services.ConfigService.String("Logo")
	aOut["Favicon"] = services.ConfigService.String("Favicon")
	aOut["Version"] = beego.AppConfig.String("version")

	// 获取控制器名称和方法名称
	controllerName, actionName := this.GetControllerAndAction()
	aOut["Controller"] = controllerName[0 : len(controllerName)-10]
	aOut["Action"] = actionName
	aOut["MethodName"] = controllerName + "." + actionName

	this.Data["aOut"] = aOut

	// 设置语言
	this.Data["Lang"] = this.Lang

	this.TplName = this.Module + "/" + tpl
}

// 设置分页
func (this *BaseController) SetPaginator(per int, nums int64) *utils.Paginator {
	p := utils.NewPaginator(this.Ctx.Request, per, nums)
	this.Data["Paginator"] = p
	return p
}

// DownloadNovel 将指定小说导出为 TXT 并作为附件下载
//
// PC 与移动端共用此方法（两端路由分别指向各自控制器的 Download，
// 内部均调用本方法），避免重复实现。
//
// 采用「边读库边写响应」的流式方式，不把整本书载入内存
// （最长的小说约 750 万字，一次性载入会带来明显的内存峰值）。
// 成功时不返回错误，仅以日志记录导出量。
func (this *BaseController) DownloadNovel(novId uint32) {
	if novId < 1 {
		this.Msg("参数错误，无法访问")
	}

	novel := services.NovelService.Get(novId)
	if novel == nil {
		this.Msg("该小说不存在或者已被删除")
	}

	if novel.ChapterNum < 1 {
		this.Msg("该小说暂无章节，无法下载")
	}

	// 文件名：书名（作者）.txt
	fileName := novel.Name
	if len(novel.Author) > 0 {
		fileName += "（" + novel.Author + "）"
	}
	fileName = sanitizeFileName(fileName) + ".txt"

	// 中文文件名兼容：filename 给 ASCII 兜底，filename* 给 UTF-8
	asciiName := "novel-" + strconv.FormatUint(uint64(novel.Id), 10) + ".txt"
	disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		asciiName, url.PathEscape(fileName))

	this.Ctx.Output.Header("Content-Type", "text/plain; charset=utf-8")
	this.Ctx.Output.Header("Content-Disposition", disposition)
	// 内容随章节更新而变化，不做长缓存
	this.Ctx.Output.Header("Cache-Control", "no-cache")

	chapNum, textNum, err := services.ChapterService.ExportNovel(
		novel.Id, novel.Name, novel.Author, this.Ctx.ResponseWriter)

	if err != nil {
		// 响应头已发出，无法再返回错误页，仅记录日志
		log.Error("导出小说失败：", novel.Name, " 已导出 ", chapNum, " 章，错误：", err.Error())
		return
	}

	log.Info("导出小说：", novel.Name, " 章节数：", chapNum, " 字数：", textNum)

	// 已直接写入响应，阻止 beego 再渲染模板
	this.StopRun()
}

// sanitizeFileName 去除文件名中的非法字符
//
// 避免书名含 / \ : * ? " < > | 等字符时导致下载文件名异常，
// 或出现路径穿越风险；同时规避 Windows 下不允许的首尾点与空格。
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "novel"
	}

	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\r", "\n", "\t"}
	for _, ch := range invalid {
		name = strings.Replace(name, ch, "_", -1)
	}

	// 去掉首尾的点与空格（Windows 下不合法）
	name = strings.Trim(name, ". ")

	// 限制长度，避免超出文件系统限制
	if utf8.RuneCountInString(name) > 80 {
		r := []rune(name)
		name = string(r[:80])
	}

	if name == "" {
		return "novel"
	}

	return name
}

type JSONResponse struct {
	Ret   interface{} `json:"ret"`
	Msg   interface{} `json:"msg"`
	Data  interface{} `json:"data"`
	URL   interface{} `json:"url"`
	IsTop interface{} `json:"is_top"`
}

// json数据输出
func (this *BaseController) OutJson(args ...interface{}) {
	json := &JSONResponse{
		Ret:   0,
		Msg:   "",
		URL:   "",
		IsTop: false,
	}

	json.Ret = args[0]
	if len(args) > 1 {
		json.Msg = args[1]
	}
	if len(args) > 2 {
		json.Data = args[2]
	}

	if len(args) > 3 {
		json.URL = args[3]
	}
	// 是否顶部跳转
	if len(args) > 4 {
		json.IsTop = args[4]
	}

	this.Data["json"] = json
	this.ServeJSON()
	this.StopRun()
}

// 提示信息
func (this *BaseController) Msg(msg string, args ...interface{}) {
	url := ""
	if len(args) > 0 {
		url = args[0].(string)
	}

	isTop := false
	if len(args) > 1 {
		isTop = args[1].(bool)
	}

	// ajax 返回json
	if this.IsAjax() {
		this.OutJson(1001, msg, url, isTop)
	}

	this.Data["Url"] = url
	this.Data["Msg"] = msg
	this.Data["Wait"] = 2
	this.Data["IsTop"] = isTop
	this.Data["Title"] = services.ConfigService.String("Title")
	this.Layout = ""
	this.TplName = "message.tpl"
	this.Render()
	this.StopRun()
}

// 去除URLFor生成的URL前缀
// 同时把前台小说的旧式地址改写为伪静态地址，
// 使模板中所有 urlfor 调用无需逐个修改即可输出 /book/1.html 形式。
func URLFor(endpoint string, values ...interface{}) string {
	url := beego.URLFor(endpoint, values...)

	if mURL := services.ConfigService.String("MobileURL"); mURL != "" {
		url = strings.Replace(url, "/m/", "/", 1)
	}

	if adminURL := services.ConfigService.String("AdminURL"); adminURL != "" {
		url = strings.Replace(url, "/admin/", "/", 1)
	}

	// 改写为伪静态地址（仅 PC 前台 /book/...）
	url = utils.PrettyURL(endpoint, url)

	return url
}
