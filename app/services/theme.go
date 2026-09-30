package services

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vckai/novel/app/utils/log"
)

/*
前台主题管理
============

背景
----
前台模板按主题分目录存放：

	views/home/{主题}/    PC 端
	views/m/{主题}/       手机端

配置项 Theme / MobileTheme 决定用哪套，控制器据此拼出模板路径：

	this.Module = "home/" + theme
	this.TplName = this.Module + "/" + tpl

但**模板文件内部的相互引用是硬编码的**，例如 layout.tpl 里：

	{{template "home/default/common/header.tpl" .}}

Go 模板的 {{template}} 不支持变量作为名字（实测会解析失败：
unexpected ".TplName" in template clause），因此引用路径必须写死主题名。
这意味着新建主题时，必须把模板内的主题名一并替换掉，否则渲染会报
「can't find templatefile」。本服务的 Create 正是自动完成这一步。

改动何时生效
------------
beego 在启动时把全部模板扫描进内存（beeTemplates），运行时只从内存取：

	if t, ok := beeTemplates[name]; ok { ... }
	panic("can't find templatefile in the path:" + name)

因此**新增/删除主题后必须重启服务**才生效。切换已有主题因只改配置项，
不需重启。前端的「需要重启」提示即源于此。
*/

// ThemeService 主题管理
type ThemeService struct{}

var Theme = &ThemeService{}

// 主题根目录（相对项目根）
const (
	themeDirPC = "views/home"
	themeDirM  = "views/m"
)

// 保留的主题名：default 是内置主题，不允许删除
const DefaultThemeName = "default"

// ThemeInfo 主题信息
type ThemeInfo struct {
	Name string // 主题目录名

	// 是否同时具备 PC 与移动端模板
	HasPC bool
	HasM  bool

	// 模板文件数
	PCFiles int
	MFiles  int

	// 是否正被使用
	IsPCTheme bool
	IsMTheme  bool
}

// 主题名允许的字符：字母、数字、下划线、短横线
//
// 限制字符集是为了防止目录穿越（如 ../）与非法路径。
var themeNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// ValidThemeName 校验主题名
func (this *ThemeService) ValidThemeName(name string) error {
	if name == "" {
		return errors.New("主题名不能为空")
	}
	if !themeNameRe.MatchString(name) {
		return errors.New("主题名只能包含字母、数字、下划线、短横线，且不超过 32 个字符")
	}
	if name == "." || name == ".." {
		return errors.New("主题名不合法")
	}
	return nil
}

// List 列出所有主题
//
// 以 PC 端目录为准枚举（以 views/home 下的子目录为集合），
// 同时标注该主题是否具备移动端模板。
func (this *ThemeService) List() []*ThemeInfo {
	pcTheme := ConfigService.String("Theme", DefaultThemeName)
	mTheme := ConfigService.String("MobileTheme", DefaultThemeName)

	names := map[string]bool{}

	for _, n := range this.subDirs(themeDirPC) {
		names[n] = true
	}
	for _, n := range this.subDirs(themeDirM) {
		names[n] = true
	}

	out := make([]*ThemeInfo, 0, len(names))

	for n := range names {
		info := &ThemeInfo{
			Name:      n,
			HasPC:     this.dirExists(filepath.Join(themeDirPC, n)),
			HasM:      this.dirExists(filepath.Join(themeDirM, n)),
			IsPCTheme: n == pcTheme,
			IsMTheme:  n == mTheme,
		}

		if info.HasPC {
			info.PCFiles = this.countTpl(filepath.Join(themeDirPC, n))
		}
		if info.HasM {
			info.MFiles = this.countTpl(filepath.Join(themeDirM, n))
		}

		out = append(out, info)
	}

	// 内置主题排最前，其余按名称排序，便于查找
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == DefaultThemeName {
			return true
		}
		if out[j].Name == DefaultThemeName {
			return false
		}
		return out[i].Name < out[j].Name
	})

	return out
}

// Create 以现有主题为基础创建新主题
//
// from 为空时以 default 为基础。创建时会自动把模板内的主题名替换为新名，
// 因为 {{template}} 引用必须与目录名一致，否则渲染时报找不到模板。
func (this *ThemeService) Create(name, from string) error {
	if err := this.ValidThemeName(name); err != nil {
		return err
	}

	if from == "" {
		from = DefaultThemeName
	}
	if err := this.ValidThemeName(from); err != nil {
		return errors.New("基础主题名不合法")
	}

	// 目标已存在则拒绝，避免覆盖掉已改好的模板
	if this.dirExists(filepath.Join(themeDirPC, name)) ||
		this.dirExists(filepath.Join(themeDirM, name)) {
		return fmt.Errorf("主题 %s 已存在", name)
	}

	// 基础主题必须存在（至少要有一套模板可复制）
	fromPC := filepath.Join(themeDirPC, from)
	fromM := filepath.Join(themeDirM, from)

	if !this.dirExists(fromPC) && !this.dirExists(fromM) {
		return fmt.Errorf("基础主题 %s 不存在", from)
	}

	createdPC, createdM := false, false

	if this.dirExists(fromPC) {
		if err := this.copyTheme(fromPC, filepath.Join(themeDirPC, name), from, name, "home"); err != nil {
			return fmt.Errorf("复制 PC 模板失败: %w", err)
		}
		createdPC = true
	}

	if this.dirExists(fromM) {
		if err := this.copyTheme(fromM, filepath.Join(themeDirM, name), from, name, "m"); err != nil {
			// PC 端已建好则回滚，避免留下半套主题
			if createdPC {
				_ = os.RemoveAll(filepath.Join(themeDirPC, name))
			}
			return fmt.Errorf("复制移动端模板失败: %w", err)
		}
		createdM = true
	}

	log.Info("[主题] 已创建:", name, " (基于", from, ")",
		" PC:", createdPC, " 移动端:", createdM)

	return nil
}

// Delete 删除主题
//
// 内置主题与正在使用的主题不允许删除。
func (this *ThemeService) Delete(name string) error {
	if err := this.ValidThemeName(name); err != nil {
		return err
	}

	if name == DefaultThemeName {
		return errors.New("内置主题 default 不允许删除")
	}

	if ConfigService.String("Theme", DefaultThemeName) == name {
		return errors.New("该主题正在被 PC 端使用，请先切换到其它主题")
	}
	if ConfigService.String("MobileTheme", DefaultThemeName) == name {
		return errors.New("该主题正在被移动端使用，请先切换到其它主题")
	}

	pc := filepath.Join(themeDirPC, name)
	m := filepath.Join(themeDirM, name)

	if !this.dirExists(pc) && !this.dirExists(m) {
		return fmt.Errorf("主题 %s 不存在", name)
	}

	if this.dirExists(pc) {
		if err := os.RemoveAll(pc); err != nil {
			return fmt.Errorf("删除 PC 模板失败: %w", err)
		}
	}
	if this.dirExists(m) {
		if err := os.RemoveAll(m); err != nil {
			return fmt.Errorf("删除移动端模板失败: %w", err)
		}
	}

	log.Info("[主题] 已删除:", name)

	return nil
}

// Switch 切换主题
//
// which 取 "pc" 或 "m"。切换只改配置项，不涉及文件，因此无需重启。
func (this *ThemeService) Switch(which, name string) error {
	if err := this.ValidThemeName(name); err != nil {
		return err
	}

	switch which {
	case "pc":
		if !this.dirExists(filepath.Join(themeDirPC, name)) {
			return fmt.Errorf("主题 %s 的 PC 模板不存在", name)
		}
		return ConfigService.SetOrCreate("Theme", name)

	case "m":
		if !this.dirExists(filepath.Join(themeDirM, name)) {
			return fmt.Errorf("主题 %s 的移动端模板不存在", name)
		}
		return ConfigService.SetOrCreate("MobileTheme", name)
	}

	return errors.New("参数错误：which 应为 pc 或 m")
}

/* ---------- 内部实现 ---------- */

// copyTheme 复制主题目录，并把模板内的主题名替换为新名
//
// tplRoot 为 "home" 或 "m"，用于构造要替换的引用前缀：
//
//	home/default/...  →  home/{name}/...
//	m/default/...     →  m/{name}/...
func (this *ThemeService) copyTheme(src, dst, from, to, tplRoot string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	oldRef := tplRoot + "/" + from + "/"
	newRef := tplRoot + "/" + to + "/"

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		// 目录：原样创建
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}

		// 非模板文件：直接复制
		if filepath.Ext(path) != ".tpl" {
			return copyFile(path, target, info.Mode().Perm())
		}

		// 模板文件：替换引用中的主题名后再写入
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		content := strings.ReplaceAll(string(data), oldRef, newRef)

		return os.WriteFile(target, []byte(content), info.Mode().Perm())
	})
}

// copyFile 复制单个文件
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)

	return err
}

// subDirs 列出目录下的子目录名
func (this *ThemeService) subDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}

	return out
}

// dirExists 目录是否存在
func (this *ThemeService) dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// countTpl 统计目录下的模板文件数（递归）
func (this *ThemeService) countTpl(dir string) int {
	n := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".tpl" {
			n++
		}
		return nil
	})
	return n
}
