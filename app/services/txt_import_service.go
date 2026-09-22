package services

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/utils/log"
)

/*
TXT 导入服务
============

把一个 txt 文件解析成章节并入库，形成一本可阅读的书。

与原采集的区别
--------------
采集的书有「采集点」，可随站点更新而增量采集；导入的书是静态的，
没有采集点，也不会被更新任务处理（更新任务是按采集点驱动的）。
为此在小说表里用 source=import 标记，便于识别与批量管理。

导入流程
--------
	1. 解析 txt → 章节列表（ParseTXT）
	2. 建/取小说记录（同名书默认复用，可指定覆盖）
	3. 逐章入库（分批，避免一次性构造过大的 SQL）
	4. 回写小说统计（章节数、字数、最新章节）

为什么走 batch 而不是逐条
------------------------
一本书动辄数百上千章，逐条插入会有大量往返。批量插入控制在 200 条一批，
与采集入库的批次保持一致。
*/

// TXTImportOptions 导入选项
type TXTImportOptions struct {
	Name   string // 书名（空则从文件名推断）
	Author string // 作者（空则从文件名推断）
	CateId uint32 // 分类 ID
	Desc   string // 简介

	Overwrite bool // 同名书已存在时，是否覆盖其章节

	// 进度回调（可为 nil）：已处理章节数、总数
	OnProgress func(done, total int)
}

// TXTImportResult 导入结果
type TXTImportResult struct {
	NovelId   uint32
	NovelName string
	Author    string
	CateId    uint32
	CateName  string
	Chapters  int  // 解析出的章节数
	Inserted  int  // 实际入库数
	TextNum   int  // 总字数
	Skipped   int  // 跳过的空章节
	IsNew     bool // 是否新建了书籍记录
	Duration  time.Duration
}

// TXTImportService TXT 导入服务
type TXTImportService struct{}

var TXTImport = &TXTImportService{}

// 导入时标记来源，便于与采集的书区分
const NovelSourceImport = "import"

// 导入相关常量
const (
	// 未指定分类时的兜底分类（与采集侧一致）
	importDefaultCate = 13

	// 简介字符上限（与 novel 表的 desc 字段定义一致）
	importMaxDescLen = 2555
)

// 每批入库的章节数
const txtImportBatch = 200

// Import 执行导入
//
// r 为 txt 文件内容。
func (this *TXTImportService) Import(r io.Reader, opt TXTImportOptions) (*TXTImportResult, error) {
	t0 := time.Now()

	// 1. 解析
	chaps, err := ParseTXT(r)
	if err != nil {
		return nil, err
	}
	if len(chaps) == 0 {
		return nil, errors.New("未解析到任何章节，请确认文件格式（需含「第X章」等章节标题）")
	}

	// 2. 书名与作者
	name := strings.TrimSpace(opt.Name)
	if name == "" {
		return nil, errors.New("书名不能为空")
	}

	author := strings.TrimSpace(opt.Author)
	if author == "" {
		author = "佚名"
	}

	// 3. 分类
	cateId := opt.CateId
	if cateId == 0 {
		cateId = importDefaultCate
	}
	cate := CateService.Get(cateId)
	if cate == nil {
		return nil, fmt.Errorf("分类不存在: %d", cateId)
	}

	res := &TXTImportResult{
		NovelName: name,
		Author:    author,
		CateId:    cateId,
		CateName:  cate.Name,
		Chapters:  len(chaps),
	}

	// 4. 建或取小说记录
	nov := NovelService.GetByName(name)
	if nov != nil && nov.Id > 0 {
		if !opt.Overwrite {
			return nil, fmt.Errorf("已存在同名小说《%s》，如需替换请勾选「覆盖已有章节」", name)
		}

		// 覆盖：清掉旧章节
		if err := ChapterService.DelByNovId(nov.Id); err != nil {
			return nil, fmt.Errorf("清理旧章节失败: %w", err)
		}
	} else {
		nov = models.NewNovel()
		nov.Name = name
		nov.Author = author
		nov.CateId = cateId
		nov.CateName = cate.Name
		nov.Status = models.BOOKFINISH // 导入的书视为完本
		nov.Desc = clipImportDesc(opt.Desc)

		if err := NovelService.Save(nov); err != nil {
			return nil, fmt.Errorf("创建小说失败: %w", err)
		}
		res.IsNew = true
	}

	res.NovelId = nov.Id

	// 5. 逐批入库
	var (
		batch    = make([]*models.Chapter, 0, txtImportBatch)
		inserted int
		textNum  int
		lastTxt  string
	)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}

		if err := ChapterService.InsertMulti(batch, true); err != nil {
			return err
		}

		inserted += len(batch)
		batch = batch[:0]

		if opt.OnProgress != nil {
			opt.OnProgress(inserted, len(chaps))
		}

		return nil
	}

	now := uint32(time.Now().Unix())

	for i, c := range chaps {
		title := strings.TrimSpace(c.Title)
		plain := strings.TrimSpace(c.Content)

		if plain == "" {
			res.Skipped++
			continue
		}

		body := TextToHTML(plain)

		chap := models.NewChapter()
		chap.NovId = nov.Id
		chap.ChapterNo = uint32(i + 1)
		chap.Title = title
		chap.Desc = body
		chap.Source = NovelSourceImport
		chap.Link = "" // 导入的书无采集链接
		chap.TextNum = uint32(len([]rune(plain)))
		chap.Status = 0
		chap.CreatedAt = now
		chap.UpdatedAt = now

		textNum += int(chap.TextNum)
		lastTxt = title

		batch = append(batch, chap)

		if len(batch) >= txtImportBatch {
			if err := flush(); err != nil {
				return nil, fmt.Errorf("入库失败: %w", err)
			}
		}
	}

	if err := flush(); err != nil {
		return nil, fmt.Errorf("入库失败: %w", err)
	}

	res.Inserted = inserted
	res.TextNum = textNum
	res.Duration = time.Since(t0)

	// 6. 回写小说统计
	nov.ChapterNum = uint32(inserted)
	nov.TextNum = uint32(textNum)
	nov.ChapterTitle = lastTxt

	if err := NovelService.UpChapterTextNum(nov.Id, textNum, false); err != nil {
		log.Warn("更新小说统计失败:", nov.Id, " ", err.Error())
	}
	if err := NovelService.UpNovelInfo(nov); err != nil {
		log.Warn("更新小说信息失败:", nov.Id, " ", err.Error())
	}

	log.Info("[TXT导入] 成功:", name, " 作者:", author,
		" 章节:", inserted, " 字数:", textNum, " 耗时:", res.Duration)

	return res, nil
}

// clipImportDesc 简介截断（与采集侧保持同一上限）
func clipImportDesc(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= importMaxDescLen {
		return s
	}
	return string(r[:importMaxDescLen])
}
