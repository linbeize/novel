package services

import (
	"strings"
	"testing"
)

// 针对 bqgnovels 规则的真实采集验证
//
// 该站目录每页只输出 100 章且无「下一页」链接，翻页靠查询参数，
// 因此重点验证 chapter_page_url_template 是否把全部章节取回。
// 需联网，默认跳过；用 go test -run TestBqgnovelsRule 显式指定即可运行。
func TestBqgnovelsRule(t *testing.T) {
	initApp()

	requireNetwork(t)

	chaps, err := SnatchService.GetChapters("bqgnovels", "https://www.bqgnovels.com/book/50049")
	if err != nil {
		t.Fatalf("目录采集失败: %v", err)
	}

	t.Logf("目录章节数: %d", len(chaps))
	if len(chaps) == 0 {
		t.Fatal("未采集到任何章节")
	}

	// 只取到 100 章说明目录分页未生效
	if len(chaps) == 100 {
		t.Errorf("只取到 100 章，说明目录分页（chapter_page_url_template）未生效")
	}

	t.Logf("首章: no=%d title=%q link=%s", chaps[0].Chap.ChapterNo, chaps[0].Chap.Title, chaps[0].Chap.Link)
	t.Logf("末章: no=%d title=%q link=%s", chaps[len(chaps)-1].Chap.ChapterNo, chaps[len(chaps)-1].Chap.Title, chaps[len(chaps)-1].Chap.Link)

	// 正文采集验证
	c, err := SnatchService.GetChapterFull("bqgnovels", "https://www.bqgnovels.com/book/50049/1")
	if err != nil {
		t.Fatalf("正文采集失败: %v", err)
	}
	if c.Chap == nil {
		t.Fatal("正文为空")
	}

	desc := strings.TrimSpace(c.Chap.Desc)
	t.Logf("正文标题: %s", c.Chap.Title)
	t.Logf("正文长度: %d 字", len([]rune(desc)))

	if len([]rune(desc)) < 500 {
		t.Errorf("正文过短（%d 字），可能选择器有误", len([]rune(desc)))
	}
}

// 检查目录里是否存在空标题/空链接的条目
func TestBqgnovelsCatalogIntegrity(t *testing.T) {
	initApp()
	requireNetwork(t)

	chaps, err := SnatchService.GetChapters("bqgnovels", "https://www.bqgnovels.com/book/50049")
	if err != nil {
		t.Fatalf("目录采集失败: %v", err)
	}

	empty := 0
	seen := map[string]int{}
	for i, c := range chaps {
		if strings.TrimSpace(c.Chap.Title) == "" || strings.TrimSpace(c.Chap.Link) == "" {
			empty++
			if empty <= 3 {
				t.Logf("第 %d 条为空: title=%q link=%q", i, c.Chap.Title, c.Chap.Link)
			}
		}
		seen[c.Chap.Link]++
	}

	dup := 0
	for u, n := range seen {
		if n > 1 {
			dup++
			if dup <= 3 {
				t.Logf("重复 URL(%d 次): %s", n, u)
			}
		}
	}

	t.Logf("总计 %d 条, 空条目 %d, 重复 URL %d 个", len(chaps), empty, dup)

	// 抽查若干章节的 URL 形态
	for _, i := range []int{0, 1, 99, 100, 500, len(chaps) - 1} {
		if i >= 0 && i < len(chaps) {
			t.Logf("  [%d] no=%d %s -> %s", i, chaps[i].Chap.ChapterNo, chaps[i].Chap.Title, chaps[i].Chap.Link)
		}
	}
}
