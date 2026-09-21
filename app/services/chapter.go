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

package services

import (
	"errors"
	"io"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/validation"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/utils/log"
)

type Chapter struct{}

func NewChapter() *Chapter {
	return &Chapter{}
}

// 获取单个章节内容
func (this *Chapter) Get(id uint64, novId uint32) *models.Chapter {
	if id < 1 {
		return nil
	}

	m := &models.Chapter{Id: id, NovId: novId}

	err := m.Read()
	if err != nil {
		return nil
	}

	return m
}

// 获取第一章节
func (this *Chapter) GetFirst(novId uint32) *models.Chapter {
	if novId < 1 {
		return nil
	}

	m := models.NewChapter()
	m.NovId = novId

	err := m.GetFirst()
	if err != nil {
		return nil
	}

	return m
}

// 获取小说上一章节
func (this *Chapter) GetPre(novId, chapNo uint32) *models.Chapter {
	if novId < 0 || chapNo < 1 {
		return nil
	}

	m := models.NewChapter()
	m.NovId = novId
	m.ChapterNo = chapNo

	err := m.GetByChapNo("pre")
	if err != nil {
		return nil
	}

	return m
}

// 获取小说下一章节
func (this *Chapter) GetNext(novId, chapNo uint32) *models.Chapter {
	if novId < 1 || chapNo < 1 {
		return nil
	}

	m := models.NewChapter()
	m.NovId = novId
	m.ChapterNo = chapNo

	err := m.GetByChapNo("next")
	if err != nil {
		return nil
	}

	return m
}

// 获取小说最新章节
func (this *Chapter) GetLast(novId uint32) *models.Chapter {
	m := models.NewChapter()

	if novId < 1 {
		return m
	}

	m.NovId = novId

	err := m.GetLast()
	if err != nil {
		return m
	}

	return m
}

// 获取小说空章节列表
func (this *Chapter) GetEmptyChaps(novId uint32) []*models.Chapter {
	if novId < 1 {
		return nil
	}

	m := models.NewChapter()
	m.NovId = novId

	// 获取小说空章节列表
	chaps := m.GetEmptyChaps()

	return chaps
}

// 获取小说章节列表
func (this *Chapter) GetNovChaps(novId uint32, size, offset int, sort string, isCount bool) ([]*models.Chapter, int64) {
	if novId < 1 {
		return nil, 0
	}

	m := models.NewChapter()
	m.NovId = novId

	// 获取小说章节列表
	chaps := m.GetNovChaps(size, offset, sort)

	// 获取小说章节数
	var count int64 = 0
	if isCount {
		count = m.Count()
	}

	return chaps, count
}

// 批量删除小说章节
func (this *Chapter) DeleteBatch(novId uint32, ids []string) error {
	if len(ids) == 0 {
		return errors.New("params error")
	}
	m := models.NewChapter()
	m.NovId = novId

	// 获取章节字数，用于更新小说主信息
	textNum := m.GetChapsTextNum(ids)

	err := m.DeleteBatch(ids)

	if err == nil {
		NovelService.UpChapterTextNum(novId, int(textNum), false)
	}

	return err
}

// 删除操作章节
func (this *Chapter) Delete(id uint64, novId uint32) error {
	if id < 1 || novId < 1 {
		return errors.New("params error")
	}

	chapter := ChapterService.Get(id, novId)

	if chapter == nil {
		return errors.New("章节不存在")
	}

	err := chapter.Delete()

	if err == nil {
		NovelService.UpChapterTextNum(novId, int(chapter.TextNum), false)
	}

	return err
}

// 清空指定小说章节
func (this *Chapter) DelByNovId(novId uint32) error {
	if novId < 0 {
		return errors.New("params error")
	}

	m := models.NewChapter()
	m.NovId = novId

	err := m.DelByNovId()

	return err
}

// 添加编辑小说章节内容
func (this *Chapter) Save(chapter *models.Chapter) error {
	// 参数校验
	valid := validation.Validation{}
	valid.Required(chapter.ChapterNo, "chapterNoEmpty").Message("章节编号不能为空")
	valid.Required(chapter.Title, "titleEmpty").Message("章节名称不能为空")
	valid.MaxSize(chapter.Title, 100, "nameMax").Message("章节名称长度不能超过100个字符")
	valid.Required(chapter.NovId, "novidEmpty").Message("所属小说不能为空")
	valid.Required(chapter.Desc, "descEmpty").Message("章节内容不能为空")

	if valid.HasErrors() {
		for _, err := range valid.Errors {
			return err
		}
	}

	novel := NovelService.Get(chapter.NovId)
	if novel == nil {
		return errors.New("小说不存在")
	}

	// 章节数统计
	textNum := utf8.RuneCountInString(chapter.Desc)

	chapter.TextNum = uint32(textNum)

	novTextNum := novel.TextNum
	chapterNum := novel.ChapterNum

	var err error
	if chapter.Id > 0 {
		err = chapter.Update("nov_id", "title", "desc", "chapter_no")
	} else {
		novTextNum += chapter.TextNum
		chapterNum++
		err = chapter.Insert()
	}

	if err != nil {
		return err
	}

	// 更新小说主信息
	NovelService.UpChapterInfo(novel.Id, int(novTextNum), int(chapterNum), chapter.Id, chapter.Title, novel.Status)

	return nil
}

// 章节浏览次数累加
func (this *Chapter) UpdateViews(chapter *models.Chapter) error {
	err := chapter.UpdateViews()

	return err
}

// 更新空章节信息
func (this *Chapter) UpdateEmpty(chapter *models.Chapter) error {
	chapter.TryViews++
	err := chapter.UpdateEmpty()

	return err
}

// 批量插入多个章节内容
func (this *Chapter) InsertMulti(chapters []*models.Chapter, isInit bool) error {

	// 非小说初始化情况，通过单条插入的方式，判断章节是否重复
	if isInit == false {
		for _, chap := range chapters {
			err := chap.Insert()
			if err != nil {
				log.Error("插入章节错误：", chap.Title, "，小说ID：", chap.NovId, "，错误：", err)
				continue
			}
		}

		return nil
	}

	max := 200
	chapLen := len(chapters)
	for i := 0; i < chapLen; i += max {
		index := int(math.Min(float64(i+max), float64(chapLen)))

		err := models.ChapterModel.InsertMulti(chapters[i:index])
		if err != nil {
			return err
		}

		time.Sleep(time.Duration(10) * time.Microsecond)
	}

	return nil
}

// 导出用的批次大小：每批读取的章节数
//
// 取 200 是折中：单批正文约 0.7MB 量级，既不会让内存占用过高，
// 也不会因批次过小而频繁查询数据库。
const exportBatchSize = 200

// ExportNovel 将整本小说导出为纯文本写入 w
//
// 设计要点：
//   - 分批读取（每批 exportBatchSize 章），处理完即释放，避免整本载入内存
//     （最长的小说约 750 万字，一次性载入会占用明显内存）
//   - 正文原为 HTML 片段，这里用 beego.HTML2str 转为纯文本，
//     并把 <br> 还原为换行，保证 TXT 排版可读
//   - 跳过采集失败的空章节，避免导出中出现大片空白
//
// 返回实际写入的章节数与字数（以中文字符计）。
func (this *Chapter) ExportNovel(novId uint32, novelName, author string, w io.Writer) (int, int, error) {
	if novId < 1 {
		return 0, 0, errors.New("小说不存在")
	}

	// 写头部信息
	var head strings.Builder
	head.WriteString(novelName)
	head.WriteString("\r\n")
	if len(author) > 0 {
		head.WriteString("作者：")
		head.WriteString(author)
		head.WriteString("\r\n")
	}
	head.WriteString("------------------------------------------------------------\r\n\r\n")

	if _, err := io.WriteString(w, head.String()); err != nil {
		return 0, 0, err
	}

	m := models.NewChapter()
	m.NovId = novId

	chapNum := 0
	textNum := 0
	afterNo := uint32(0)

	for {
		batch := m.GetNovChapsWithContent(afterNo, exportBatchSize)
		if len(batch) == 0 {
			break
		}

		for _, chap := range batch {
			afterNo = chap.ChapterNo

			title := strings.TrimSpace(chap.Title)
			body := htmlToText(chap.Desc)

			// 空章节（采集失败）跳过，仅保留标题占位
			if len(strings.TrimSpace(body)) == 0 {
				continue
			}

			var sb strings.Builder
			sb.WriteString(title)
			sb.WriteString("\r\n\r\n")
			sb.WriteString(body)
			sb.WriteString("\r\n\r\n")

			if _, err := io.WriteString(w, sb.String()); err != nil {
				return chapNum, textNum, err
			}

			chapNum++
			textNum += utf8.RuneCountInString(body)
		}

		// 本批不足一批，说明已到末尾
		if len(batch) < exportBatchSize {
			break
		}
	}

	return chapNum, textNum, nil
}

// htmlToText 把章节正文（HTML 片段）转为纯文本
//
// 采集到的正文形如：
//
//	<br/>&nbsp;&nbsp;正文第一段<br/><br/>&nbsp;&nbsp;正文第二段
//
// 处理步骤：
//  1. <br> / <br/> / </p> 等换行标记替换为 \n
//  2. 去除其余 HTML 标签
//  3. 反转义实体（&nbsp; &amp; 等）
//  4. 规范空白：去掉首尾空白、压缩连续空行为单个空行
func htmlToText(s string) string {
	if len(s) == 0 {
		return ""
	}

	// 1. 换行标记
	replacer := strings.NewReplacer(
		"<br/>", "\n",
		"<br />", "\n",
		"<br>", "\n",
		"</p>", "\n",
		"</div>", "\n",
	)
	s = replacer.Replace(s)

	// 2. 去标签 + 3. 反转义
	s = beego.HTML2str(s)

	// 4. 规范化空白
	s = strings.Replace(s, "\u00a0", " ", -1)
	s = strings.Replace(s, "\r\n", "\n", -1)
	s = strings.Replace(s, "\r", "\n", -1)

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			blank++
			// 连续空行压缩为一个
			if blank > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blank = 0
		out = append(out, ln)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}
