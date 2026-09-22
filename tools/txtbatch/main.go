package main

/*
批量导入知轩藏书 TXT 全集
========================

用途
----
把一个 rar/txt 合集批量导入站内。相比后台的「TXT 导入」页面，
本工具适合一次导入数百上千本的场景（后台逐本上传不现实）。

用法
----
	# 先小批量试跑（默认 5 本），确认解析与入库正常
	go run ./tools/txtbatch -rar=<rar路径> -limit=5 -dry

	# 确认无误后全量导入
	go run ./tools/txtbatch -rar=<rar路径>

设计要点
--------
  - 逐本解压到内存解析，不落盘（合集有十几 GB，全部解压会撑爆磁盘）
  - 每本处理完立即入库并释放，内存占用与单本书相当
  - 已存在的书默认跳过，因此中断后可直接重跑续传
  - -dry 只解析不入库，用于先看解析效果
*/

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/astaxie/beego"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services"
)

var (
	rarPath      = flag.String("rar", "", "rar 文件路径（必填）")
	limit        = flag.Int("limit", 0, "最多处理多少本（0=全部），用于试跑")
	dry          = flag.Bool("dry", false, "只解析不入库，打印解析结果")
	listFile     = flag.String("list", "", "从文件读取待处理列表（每行一个文件名）")
	cateId       = flag.Uint("cate", 13, "默认分类 ID")
	skipExisting = flag.Bool("skip", true, "已存在的书跳过（便于中断后续传）")
)

// 标准命名：《书名》（版本说明）作者：作者名.txt
var nameRe = regexp.MustCompile(`^《(.+?)》(（([^）]*)）)?作者：(.+?)\.txt$`)

func main() {
	flag.Parse()

	if *rarPath == "" {
		fmt.Println("请用 -rar 指定 rar 文件路径")
		os.Exit(1)
	}

	initApp()

	// 取文件列表
	names, err := readList()
	if err != nil {
		fmt.Println("读取列表失败:", err)
		os.Exit(1)
	}

	fmt.Printf("清单共 %d 个文件\n", len(names))
	if *limit > 0 && len(names) > *limit {
		names = names[:*limit]
		fmt.Printf("本次处理前 %d 个（试跑模式）\n", len(names))
	}
	if *dry {
		fmt.Println("模式: 只解析不入库")
	}

	var (
		ok, skip, fail int
		totalChaps     int
		totalText      int
		failSamples    []string
	)

	t0 := time.Now()

	for _, fn := range names {
		book, author, ver := parseName(fn)

		if book == "" {
			skip++
			continue
		}

		// 已存在则跳过
		if *skipExisting && !*dry {
			if n := services.NovelService.GetByName(book); n != nil && n.Id > 0 {
				skip++
				continue
			}
		}

		// 解压到内存
		content, err := extract(fn)
		if err != nil {
			fail++
			if len(failSamples) < 10 {
				failSamples = append(failSamples, fmt.Sprintf("%s: 解压失败 %v", fn, err))
			}
			continue
		}

		// 解析
		chaps, err := services.ParseTXT(strings.NewReader(content))
		if err != nil || len(chaps) == 0 {
			fail++
			if len(failSamples) < 10 {
				failSamples = append(failSamples, fmt.Sprintf("%s: 解析失败(%d章) %v", fn, len(chaps), err))
			}
			continue
		}

		textNum := 0
		for _, c := range chaps {
			textNum += len([]rune(c.Content))
		}

		if *dry {
			ok++
			totalChaps += len(chaps)
			totalText += textNum
			if ok <= 20 || ok%100 == 0 {
				fmt.Printf("  [%d/%d] 《%s》%s  %d章 %d字%s\n",
					ok, len(names), book, author, len(chaps), textNum, verTag(ver))
			}
			continue
		}

		// 入库
		res, err := services.TXTImport.Import(strings.NewReader(content), services.TXTImportOptions{
			Name:   book,
			Author: author,
			CateId: uint32(*cateId),
		})
		if err != nil {
			fail++
			if len(failSamples) < 10 {
				failSamples = append(failSamples, fmt.Sprintf("%s: 入库失败 %v", fn, err))
			}
			continue
		}

		ok++
		totalChaps += res.Inserted
		totalText += res.TextNum

		if ok <= 20 || ok%50 == 0 {
			fmt.Printf("  [%d] 《%s》%s  %d章 %d字 (%.1fs)\n",
				ok, book, author, res.Inserted, res.TextNum, time.Since(t0).Seconds())
		}
	}

	fmt.Println()
	fmt.Println("========== 完成 ==========")
	fmt.Printf("成功: %d 本\n", ok)
	fmt.Printf("跳过: %d 本（已存在或文件名无法解析）\n", skip)
	fmt.Printf("失败: %d 本\n", fail)
	fmt.Printf("章节: %d 章\n", totalChaps)
	fmt.Printf("字数: %d\n", totalText)
	fmt.Printf("耗时: %v\n", time.Since(t0))

	if len(failSamples) > 0 {
		fmt.Println()
		fmt.Println("失败样例:")
		for _, s := range failSamples {
			fmt.Println("  ", s)
		}
	}
}

// readList 读待处理文件名列表
func readList() ([]string, error) {
	// 优先从 -list 指定的文件读
	if *listFile != "" {
		b, err := os.ReadFile(*listFile)
		if err != nil {
			return nil, err
		}
		return splitLines(string(b)), nil
	}

	// 否则现场列 rar
	cmd := exec.Command("unrar", "lb", *rarPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("列出 rar 内容失败: %w", err)
	}

	return splitLines(string(out)), nil
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// extract 解压单个文件到内存
func extract(name string) (string, error) {
	cmd := exec.Command("unrar", "p", "-inul", *rarPath, name)

	var buf strings.Builder
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// parseName 解析文件名
//
// 标准形态：《书名》（版本说明）作者：作者名.txt
// 兼容若干不规范写法（缺作者、缺书名号等）。
func parseName(fn string) (book, author, ver string) {
	// 只取文件名部分
	fn = filepath.Base(fn)

	if m := nameRe.FindStringSubmatch(fn); m != nil {
		book = strings.TrimSpace(m[1])
		ver = strings.TrimSpace(m[3])
		author = strings.TrimSpace(m[4])
		return
	}

	// 去掉扩展名
	base := strings.TrimSuffix(fn, filepath.Ext(fn))
	base = strings.TrimSuffix(base, ".TXT")

	// 「书名作者：作者名」
	if i := strings.Index(base, "作者："); i > 0 {
		book = strings.Trim(strings.TrimSpace(base[:i]), "《》")
		author = strings.TrimSpace(base[i+len("作者："):])
		return
	}

	// 无作者信息：去掉版本括号后整段作书名
	book = strings.TrimSpace(base)
	book = strings.Trim(book, "《》")

	if i := strings.Index(book, "（"); i > 0 {
		ver = strings.Trim(book[i:], "（）")
		book = strings.TrimSpace(book[:i])
	}

	return
}

func verTag(v string) string {
	if v == "" {
		return ""
	}
	return " [" + v + "]"
}

// initApp 初始化应用依赖
func initApp() {
	// 切到项目根目录，使 beego 能读到 conf/app.conf
	if wd, err := os.Getwd(); err == nil {
		for i := 0; i < 4; i++ {
			if _, err := os.Stat(filepath.Join(wd, "conf", "app.conf")); err == nil {
				_ = os.Chdir(wd)
				break
			}
			wd = filepath.Dir(wd)
		}
	}

	beego.BConfig.RunMode = "prod"
	beego.LoadAppConfig("ini", "conf/app.conf")

	models.InitDB()
	services.Init()
}

var _ = io.Discard
