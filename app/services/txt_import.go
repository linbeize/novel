package services

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/axgle/mahonia"
)

/*
TXT 小说导入
============

用于把用户手上的 txt 小说文件导入到站内，使没有采集源的书也能上架。

文本格式
--------
中文小说 txt 没有统一规范，实际见到的形态主要有：

	第一章 章节名          ← 最常见
	第1章 章节名
	第001章 章节名
	第1节 章节名
	第1回 章节名
	Chapter 1 章节名
	序章 章节名 / 楔子 / 番外 / 后记
	卷一 标题              ← 分卷标题（不算正文章节）

因此标题识别采用「前缀特征 + 长度约束」：
  - 必须以「第N章/节/回」或「序章/楔子/番外」等起头
  - 整行要足够短（默认 50 字以内）——正文里出现「第一章」三字的长句不算标题
  - 该行不能以句末标点结尾

编码
----
绝大多数是 UTF-8 或 GBK/GB18030。导入时自动探测：
  先按 UTF-8 校验，失败则按 GB18030 解码。

正文格式
--------
库中 desc 统一为「含 <br/> 的 HTML 片段」（与采集入库的格式一致），
因此导入时把纯文本按行转成该格式，保证前台模板渲染一致。
*/

// ChapterParseResult 解析结果
type ChapterParseResult struct {
	Title   string // 章节标题
	Content string // 正文（纯文本）
}

// txtHeaderRe 章节标题的前缀特征
//
// 匹配形如：第一章 / 第1章 / 第001节 / 第1回 / 序章 / 楔子 / 番外 / 后记 / 尾声
var txtHeaderRe = regexp.MustCompile(
	`^\s*(第\s*[0-9一二三四五六七八九十百千万零两]+\s*[章节回卷部篇]|序[章言]|楔子|引子|前言|后记|尾声|番外|终章|大结局)`,
)

// txtVolRe 分卷标题（单独成行的「第N卷 ...」）
//
// 分卷标题不作为章节，但其后的内容归属下一章，故解析时跳过该行。
var txtVolRe = regexp.MustCompile(
	`^\s*第\s*[0-9一二三四五六七八九十百千万零两]+\s*[卷部篇]\s*`,
)

const (
	// 标题行最大长度（超出则认为是正文中的引用，不是标题）
	txtTitleMaxLen = 50

	// 标题行最小长度
	txtTitleMinLen = 2
)

// ParseTXT 解析 txt 小说内容为章节列表
//
// r 为文件内容；返回解析出的章节（不含书名与作者，二者由调用方另行提供
// 或从文件头部推断）。
func ParseTXT(r io.Reader) ([]ChapterParseResult, error) {
	// 读全部内容并探测编码
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	text := decodeTXT(raw)

	// 统一换行
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// 去掉 BOM 残留
	text = strings.TrimPrefix(text, "\ufeff")

	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 容忍超长行

	var (
		chapters []ChapterParseResult
		cur      *ChapterParseResult
		body     strings.Builder
	)

	flush := func() {
		if cur == nil {
			return
		}
		cur.Content = strings.TrimSpace(body.String())
		chapters = append(chapters, *cur)
		body.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// 空行保留为段落分隔
		if trimmed == "" {
			if cur != nil {
				body.WriteString("\n")
			}
			continue
		}

		// 分卷标题：跳过，不新建章节
		if txtVolRe.MatchString(trimmed) && !txtHeaderRe.MatchString(trimmed) {
			continue
		}

		// 是否章节标题
		if isTXTChapterTitle(trimmed) {
			flush()
			cur = &ChapterParseResult{Title: trimmed}
			continue
		}

		// 正文
		if cur != nil {
			body.WriteString(line)
			body.WriteString("\n")
		} else {
			// 首个标题之前的内容（封面文案、书名、简介等）丢弃
			continue
		}
	}

	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取内容失败: %w", err)
	}

	// 过滤掉正文为空的章节（多为误识别的标题行）
	out := make([]ChapterParseResult, 0, len(chapters))
	for _, c := range chapters {
		if strings.TrimSpace(c.Content) == "" {
			continue
		}
		out = append(out, c)
	}

	return out, nil
}

// isTXTChapterTitle 判断该行是否为章节标题
func isTXTChapterTitle(line string) bool {
	if !txtHeaderRe.MatchString(line) {
		return false
	}

	r := []rune(line)
	if len(r) < txtTitleMinLen || len(r) > txtTitleMaxLen {
		return false
	}

	// 标题不应以句末标点结尾（那种多是正文里提到「第一章」的长句）
	switch r[len(r)-1] {
	case '。', '！', '？', '；', '，', '、', '…':
		return false
	}

	// 标题中不应含「。」——含则基本可判定是被截断的正文
	if strings.Contains(line, "。") {
		return false
	}

	return true
}

// decodeTXT 探测并解码 txt 内容
//
// 顺序：UTF-8 校验 → 失败则按 GB18030（兼容 GBK/GB2312）。
func decodeTXT(raw []byte) string {
	// 去掉 UTF-8 BOM
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	if utf8.Valid(raw) {
		return string(raw)
	}

	dec := mahonia.NewDecoder("GB18030")
	if dec == nil {
		return string(raw)
	}

	return dec.ConvertString(string(raw))
}

// TextToHTML 把纯文本正文转为库中统一的 HTML 片段格式
//
// 与采集入库的 desc 保持一致（逐行转为「　　xxx<br/>」），
// 这样前台模板无需为新导入的书做特殊处理。
func TextToHTML(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	var sb strings.Builder
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}

		sb.WriteString("　　")
		sb.WriteString(escapeHTML(ln))
		sb.WriteString("<br/>")
	}

	return sb.String()
}

// escapeHTML 转义正文中的 HTML 特殊字符
func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

// GuessBookName 从文件名推断书名
//
// 常见形态：「书名.txt」「书名 作者.txt」「[作者]书名.txt」「书名（完结）.txt」
func GuessBookName(filename string) (name, author string) {
	// 去掉扩展名与路径
	if i := strings.LastIndex(filename, "/"); i >= 0 {
		filename = filename[i+1:]
	}
	if i := strings.LastIndex(filename, "."); i > 0 {
		filename = filename[:i]
	}

	name = strings.TrimSpace(filename)

	// 去常见后缀标记
	for _, suf := range []string{"（完结）", "(完结)", "【完结】", "[完结]", "全本", "完本"} {
		name = strings.TrimSpace(strings.TrimSuffix(name, suf))
	}

	// 「书名 作者」形态
	//
	// 只认空格分隔，不认连字符：不少书名本身含「-」（如「大侠魂-花间浪子」），
	// 若按连字符拆会把完整书名截断。空格则是常见的分隔写法，
	// 且配合右侧长度与字符检查足以区分。
	for _, sep := range []string{" ", "　"} {
		if i := strings.LastIndex(name, sep); i > 0 {
			left := strings.TrimSpace(name[:i])
			right := strings.TrimSpace(name[i+len(sep):])

			if left != "" && looksLikeAuthor(right) {
				return left, right
			}
		}
	}

	// 「[作者]书名」形态
	if strings.HasPrefix(name, "[") {
		if i := strings.Index(name, "]"); i > 0 && i < len(name)-1 {
			return strings.TrimSpace(name[i+1:]), strings.TrimSpace(name[1:i])
		}
	}

	return name, ""
}

// looksLikeAuthor 判断是否为作者名
//
// 用于把「书名 作者」拆开。判据偏保守，避免把书名本身当成作者：
//   - 长度 2-6 字（中文姓名常见长度）
//   - 只含中文、英文字母、点号（笔名偶有「·」）
//   - 不含数字与其余标点
func looksLikeAuthor(s string) bool {
	r := []rune(s)

	if len(r) < 2 || len(r) > 6 {
		return false
	}

	for _, c := range r {
		switch {
		case c >= 0x4e00 && c <= 0x9fa5: // 汉字
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case c == '·' || c == '.':
		default:
			return false
		}
	}

	return true
}
