package admin

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/astaxie/beego"

	"github.com/vckai/novel/app/services"
)

/*
TXT 导入控制器
==============

上传 txt 文件导入为站内小说。适合以下场景：
  - 手上已有 txt 文件，但该书的采集源已失效
  - 自写/自校的书稿
  - 采集源里没有的书

页面流程：
	1. 选择 txt 文件
	2. 填写书名、作者、分类（书名与作者可从文件名自动推断）
	3. 提交后即解析入库，页面显示结果

同名书处理：默认拒绝，需显式勾选「覆盖已有章节」才会替换，
避免误操作把已采好的书清空。
*/

// TxtImportController TXT 导入
type TxtImportController struct {
	BaseController
}

// 上传文件的临时目录（导入完成后删除）
const txtImportTmpDir = "data/txtimport"

// Index 导入页面
func (this *TxtImportController) Index() {
	this.Data["Cates"] = services.CateService.GetAll()
	this.View("txtimport/index.tpl")
}

// Upload 处理上传与导入
func (this *TxtImportController) Upload() {
	// 1. 取上传文件
	file, header, err := this.GetFile("txtfile")
	if err != nil {
		this.OutJson(1001, "上传失败："+err.Error())
		return
	}
	defer file.Close()

	if header == nil || header.Filename == "" {
		this.OutJson(1002, "请选择要导入的 txt 文件")
		return
	}

	// 仅接受 txt（按扩展名判断；内容格式由解析器负责）
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" {
		this.OutJson(1003, "仅支持 txt 文件，当前为 "+ext)
		return
	}

	// 限制文件大小，避免一次性读入过大内容
	const maxSize = 50 << 20 // 50MB
	if header.Size > maxSize {
		this.OutJson(1004, "文件过大，请控制在 50MB 以内")
		return
	}

	// 2. 落盘到临时目录
	if err := os.MkdirAll(txtImportTmpDir, 0755); err != nil {
		this.OutJson(1005, "创建临时目录失败："+err.Error())
		return
	}

	tmpName := filepath.Join(txtImportTmpDir,
		time.Now().Format("20060102150405")+"_"+filepath.Base(header.Filename))

	if err := this.SaveToFile("txtfile", tmpName); err != nil {
		this.OutJson(1006, "保存文件失败："+err.Error())
		return
	}
	defer os.Remove(tmpName)

	// 3. 解析选项
	// 书名与作者留空时由文件名推断
	guessName, guessAuthor := services.GuessBookName(header.Filename)

	name := strings.TrimSpace(this.GetString("name"))
	if name == "" {
		name = guessName
	}

	author := strings.TrimSpace(this.GetString("author"))
	if author == "" {
		author = guessAuthor
	}

	cateId, _ := this.GetUint32("cate_id", 0)
	if cateId == 0 {
		cateId = 13 // 其他类型
	}

	overwrite, _ := this.GetBool("overwrite", false)

	// 4. 执行导入
	f, err := os.Open(tmpName)
	if err != nil {
		this.OutJson(1007, "读取文件失败："+err.Error())
		return
	}
	defer f.Close()

	res, err := services.TXTImport.Import(f, services.TXTImportOptions{
		Name:      name,
		Author:    author,
		CateId:    cateId,
		Desc:      strings.TrimSpace(this.GetString("desc")),
		Overwrite: overwrite,
	})
	if err != nil {
		this.OutJson(1008, "导入失败："+err.Error())
		return
	}

	this.AddLog(3303)

	this.OutJson(0, "导入成功", map[string]interface{}{
		"novel_id":   res.NovelId,
		"novel_name": res.NovelName,
		"author":     res.Author,
		"cate_name":  res.CateName,
		"chapters":   res.Inserted,
		"text_num":   res.TextNum,
		"duration":   res.Duration.String(),
		"is_new":     res.IsNew,
		"url":        beego.AppConfig.String("adminurl"),
	})
}

// Parse 仅解析不入库
//
// 用于导入前预览：让用户确认章节切分是否正确，避免切错后还要删书重来。
func (this *TxtImportController) Parse() {
	file, header, err := this.GetFile("txtfile")
	if err != nil {
		this.OutJson(1001, "上传失败："+err.Error())
		return
	}
	defer file.Close()

	if header == nil || header.Filename == "" {
		this.OutJson(1002, "请选择要导入的 txt 文件")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" {
		this.OutJson(1003, "仅支持 txt 文件")
		return
	}

	chaps, err := services.ParseTXT(file)
	if err != nil {
		this.OutJson(1004, "解析失败："+err.Error())
		return
	}

	// 只回传预览信息（不传全部正文，避免响应过大）
	preview := make([]map[string]interface{}, 0, 10)
	totalText := 0

	for i, c := range chaps {
		totalText += len([]rune(c.Content))

		if i < 10 {
			body := []rune(strings.TrimSpace(c.Content))
			n := 60
			if len(body) < n {
				n = len(body)
			}

			preview = append(preview, map[string]interface{}{
				"no":    i + 1,
				"title": c.Title,
				"body":  string(body[:n]),
				"len":   len(body),
			})
		}
	}

	name, author := services.GuessBookName(header.Filename)

	this.OutJson(0, "解析成功", map[string]interface{}{
		"chapters":   len(chaps),
		"text_num":   totalText,
		"preview":    preview,
		"guess_name": name,
		"guess_auth": author,
	})
}
