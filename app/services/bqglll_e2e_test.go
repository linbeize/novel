package services

import "testing"

// 接口型采集器端到端测试
func TestBqglllE2E(t *testing.T) {
	requireNetwork(t)

	// 1) 搜索
	info, err := SnatchService.FindNovel("bqglll", "机武风暴")
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	t.Logf("搜索: %s / %s / %s", info.Nov.Name, info.Nov.Author, info.Nov.CateName)
	t.Logf("  采集点: %s", info.Url)
	t.Logf("  封面: %s", info.Nov.Cover)

	// 2) 详情
	nov, err := SnatchService.GetNovel("bqglll", info.Url)
	if err != nil {
		t.Fatalf("详情失败: %v", err)
	}
	t.Logf("详情: %s, 状态=%d, 最新章节=%s", nov.Nov.Name, nov.Nov.Status, nov.Nov.ChapterTitle)
	t.Logf("  简介: %.50s", nov.Nov.Desc)

	// 3) 目录
	chaps, err := SnatchService.GetChapters("bqglll", info.Url)
	if err != nil {
		t.Fatalf("目录失败: %v", err)
	}
	t.Logf("目录: %d 章", len(chaps))
	if len(chaps) > 0 {
		t.Logf("  首章: no=%d %s", chaps[0].Chap.ChapterNo, chaps[0].Chap.Title)
		t.Logf("  链接: %s", chaps[0].Chap.Link)
	}

	// 4) 正文
	ch, err := SnatchService.GetChapterFull("bqglll", chaps[0].Chap.Link)
	if err != nil {
		t.Fatalf("正文失败: %v", err)
	}
	t.Logf("正文: %s", ch.Chap.Title)
	t.Logf("  长度: %d 字符", len([]rune(ch.Chap.Desc)))
	t.Logf("  预览: %.80s", ch.Chap.Desc)

	// 5) 末章（验证边界）
	last := chaps[len(chaps)-1].Chap.Link
	ch2, err := SnatchService.GetChapterFull("bqglll", last)
	if err != nil {
		t.Fatalf("末章失败: %v", err)
	}
	t.Logf("末章: %s, %d 字符", ch2.Chap.Title, len([]rune(ch2.Chap.Desc)))
}
