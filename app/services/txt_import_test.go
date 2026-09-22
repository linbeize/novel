package services

import (
	"strings"
	"testing"
)

// TXT 解析：各类格式样本
func TestParseTXT(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantChaps int
		wantFirst string
	}{
		{
			name: "标准格式-第X章",
			content: `书名：测试小说
作者：测试作者

第一章 开始

这是第一章的正文。
第二行内容。

第二章 继续

这是第二章的正文。
`,
			wantChaps: 2,
			wantFirst: "第一章 开始",
		},
		{
			name: "阿拉伯数字-第1章",
			content: `第1章 序曲

正文一。

第2章 发展

正文二。
`,
			wantChaps: 2,
			wantFirst: "第1章 序曲",
		},
		{
			name: "带前导零-第001章",
			content: `第001章 开端

正文一。

第002章 深入

正文二。
`,
			wantChaps: 2,
			wantFirst: "第001章 开端",
		},
		{
			name: "回目格式-第X回",
			content: `第一回 楔子

正文一。

第二回 相遇

正文二。
`,
			wantChaps: 2,
			wantFirst: "第一回 楔子",
		},
		{
			name: "序章与番外",
			content: `序章

序章内容。

第一章 正文开始

正文内容。

番外 后日谈

番外内容。
`,
			wantChaps: 3,
			wantFirst: "序章",
		},
		{
			name: "分卷标题应跳过",
			content: `第一卷 风起云涌

第一章 少年

正文一。

第二卷 暗流涌动

第二章 成长

正文二。
`,
			wantChaps: 2,
			wantFirst: "第一章 少年",
		},
		{
			name: "正文中出现『第一章』不应误判",
			content: `第一章 真正开始

他翻开书，看到上面写着第一章的内容简介，这一章讲的是主角的成长历程和冒险故事的开端。

第二章 继续

另一段正文。
`,
			wantChaps: 2,
			wantFirst: "第一章 真正开始",
		},
		{
			name: "标题带句号不应误判",
			content: `第一章 开始

正文一。这里提到第一章。这个句子很长很长很长很长很长很长很长很长很长很长很长很长。

第二章 结束

正文二。
`,
			wantChaps: 2,
			wantFirst: "第一章 开始",
		},
		{
			name: "空章节应被过滤",
			content: `第一章 有内容

正文内容。

第二章 空章节

第三章 也有内容

正文内容。
`,
			wantChaps: 2, // 空章节被过滤
			wantFirst: "第一章 有内容",
		},
	}

	for _, c := range cases {
		chaps, err := ParseTXT(strings.NewReader(c.content))
		if err != nil {
			t.Errorf("  %s: 解析失败 %v", c.name, err)
			continue
		}

		if len(chaps) != c.wantChaps {
			t.Errorf("  %s: 章节数 %d，期望 %d", c.name, len(chaps), c.wantChaps)
			for i, ch := range chaps {
				t.Logf("      [%d] %q", i, ch.Title)
			}
			continue
		}

		if len(chaps) > 0 && chaps[0].Title != c.wantFirst {
			t.Errorf("  %s: 首章 %q，期望 %q", c.name, chaps[0].Title, c.wantFirst)
			continue
		}

		t.Logf("  ✅ %-26s %d 章，首章 %q", c.name, len(chaps), c.wantFirst)
	}
}

// 编码探测：GBK 内容应能正确解码
func TestDecodeTXTGBK(t *testing.T) {
	// 「第一章 测试」的 GBK 编码
	gbk := []byte{
		0xB5, 0xDA, 0xD2, 0xBB, 0xD5, 0xC2, 0x20, 0xB2, 0xE2, 0xCA, 0xD4,
		0x0D, 0x0A, 0x0D, 0x0A,
		0xD5, 0xFD, 0xCE, 0xC4, 0xC4, 0xDA, 0xC8, 0xDD, 0xA1, 0xA3,
	}

	got := decodeTXT(gbk)
	t.Logf("  GBK 解码结果: %q", got)

	if !strings.Contains(got, "第一章") {
		t.Errorf("  ❌ GBK 解码失败")
	} else {
		t.Logf("  ✅ GBK 内容正确解码")
	}
}

// UTF-8 内容不应被二次解码
func TestDecodeTXTUTF8(t *testing.T) {
	utf8Content := "第一章 测试\n\n正文内容。"
	got := decodeTXT([]byte(utf8Content))

	if got != utf8Content {
		t.Errorf("  ❌ UTF-8 内容被改动: %q", got)
	} else {
		t.Logf("  ✅ UTF-8 内容原样保留")
	}
}

// 带 BOM 的 UTF-8
func TestDecodeTXTBOM(t *testing.T) {
	raw := append([]byte{0xEF, 0xBB, 0xBF}, []byte("第一章 测试")...)
	got := decodeTXT(raw)

	if strings.HasPrefix(got, "\ufeff") {
		t.Errorf("  ❌ BOM 未去除")
	} else {
		t.Logf("  ✅ BOM 已去除: %q", got)
	}
}

// 书名推断
func TestGuessBookName(t *testing.T) {
	cases := []struct{ in, wantName, wantAuthor string }{
		{"斗破苍穹.txt", "斗破苍穹", ""},
		{"斗破苍穹 天蚕土豆.txt", "斗破苍穹", "天蚕土豆"},
		{"[天蚕土豆]斗破苍穹.txt", "斗破苍穹", "天蚕土豆"},
		{"斗破苍穹（完结）.txt", "斗破苍穹", ""},
		// 连字符不拆分：书名本身可能含「-」（如「大侠魂-花间浪子」）
		{"斗破苍穹-天蚕土豆.txt", "斗破苍穹-天蚕土豆", ""},
		{"大侠魂-花间浪子.txt", "大侠魂-花间浪子", ""},
		{"/path/to/斗破苍穹.txt", "斗破苍穹", ""},
	}

	for _, c := range cases {
		name, author := GuessBookName(c.in)
		if name != c.wantName || author != c.wantAuthor {
			t.Errorf("  %q → (%q, %q)，期望 (%q, %q)", c.in, name, author, c.wantName, c.wantAuthor)
		} else {
			t.Logf("  ✅ %-28s → 书名 %q 作者 %q", c.in, name, author)
		}
	}
}

// 正文转 HTML 片段
func TestTextToHTML(t *testing.T) {
	got := TextToHTML("第一行\n\n第二行\n第三行")
	want := "　　第一行<br/>　　第二行<br/>　　第三行<br/>"

	if got != want {
		t.Errorf("  转换结果不符:\n    实际 %q\n    期望 %q", got, want)
	} else {
		t.Logf("  ✅ %q", got)
	}

	// HTML 特殊字符应转义
	got2 := TextToHTML("含 <script> 标签")
	if strings.Contains(got2, "<script>") {
		t.Errorf("  ❌ HTML 未转义: %q", got2)
	} else {
		t.Logf("  ✅ HTML 已转义: %q", got2)
	}
}
