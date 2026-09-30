package admin

import (
	"github.com/vckai/novel/app/services"
)

/*
主题管理控制器
==============

功能：列出可用主题、切换、基于现有主题创建新主题、删除。

关于「重启生效」
----------------
beego 在启动时把全部模板扫描进内存，运行时只从内存取（取不到会 panic）。
因此**新增或删除主题后必须重启服务**；而切换主题只改配置项，立即生效。

页面上的提示即据此区分，避免用户以为切换没生效。
*/

// ThemeController 主题管理
type ThemeController struct {
	BaseController
}

// Index 主题列表
func (this *ThemeController) Index() {
	themes := services.Theme.List()

	// 统计有多少主题具备完整模板（PC + 移动端）
	complete := 0
	for _, t := range themes {
		if t.HasPC && t.HasM {
			complete++
		}
	}

	this.Data["Themes"] = themes
	this.Data["ThemeCount"] = len(themes)
	this.Data["CompleteCount"] = complete

	// 当前使用的主题
	this.Data["PCTheme"] = services.ConfigService.String("Theme", services.DefaultThemeName)
	this.Data["MTheme"] = services.ConfigService.String("MobileTheme", services.DefaultThemeName)

	this.View("theme/index.tpl")
}

// Switch 切换主题
//
// 只改配置项，不涉及文件，立即生效（无需重启）。
func (this *ThemeController) Switch() {
	which := this.GetString("which")
	name := this.GetString("name")

	if err := services.Theme.Switch(which, name); err != nil {
		this.OutJson(1001, err.Error())
		return
	}

	side := "PC 端"
	if which == "m" {
		side = "移动端"
	}

	this.AddLog(3304)

	this.OutJson(0, "已切换到 "+name+"（"+side+"），刷新前台即可看到")
}

// Create 创建主题
func (this *ThemeController) Create() {
	name := this.GetString("name")
	from := this.GetString("from")

	if err := services.Theme.Create(name, from); err != nil {
		this.OutJson(1001, err.Error())
		return
	}

	this.AddLog(3304)

	// 新建主题后模板尚未载入内存，必须重启才能使用
	this.OutJson(0, "主题 "+name+" 已创建。新主题需要重启服务后才会生效。")
}

// Delete 删除主题
func (this *ThemeController) Delete() {
	name := this.GetString("name")

	if err := services.Theme.Delete(name); err != nil {
		this.OutJson(1001, err.Error())
		return
	}

	this.AddLog(3304)

	this.OutJson(0, "主题 "+name+" 已删除。需重启服务后生效。")
}
