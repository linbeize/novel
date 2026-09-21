package bqglll

import (
	"encoding/json"
	"testing"
)

// dirid 的类型不固定：等于书籍 ID 时返回数字，不同时返回字符串。
// 这个测试锁定该兼容行为，避免以后又因类型变化导致解析失败。
func TestFlexString(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`"301"`, "301"}, // 字符串形态
		{`301`, "301"},   // 数字形态
		{`"113680"`, "113680"},
		{`113680`, "113680"},
		{`null`, ""}, // 空值
	}

	for _, c := range cases {
		var f FlexString
		if err := json.Unmarshal([]byte(c.raw), &f); err != nil {
			t.Errorf("解析 %s 失败: %v", c.raw, err)
			continue
		}
		if f.String() != c.want {
			t.Errorf("解析 %s = %q, 期望 %q", c.raw, f.String(), c.want)
		}
	}
}

// 用真实的两种响应结构验证整体解析
func TestChapterUnmarshalBothTypes(t *testing.T) {
	// dirid 为字符串（dirid != id 的书）
	strForm := `{"id":4767,"chapterid":1,"dirid":"301","chaptername":"第1章","cs":1242,"txt":"正文"}`
	var c1 Chapter
	if err := json.Unmarshal([]byte(strForm), &c1); err != nil {
		t.Fatalf("字符串 dirid 解析失败: %v", err)
	}
	if c1.DirId.String() != "301" || c1.ChapterName != "第1章" {
		t.Errorf("解析结果不符: %+v", c1)
	}

	// dirid 为数字（dirid == id 的书）
	numForm := `{"id":113680,"chapterid":1,"dirid":113680,"chaptername":"第一章","cs":688,"txt":"正文"}`
	var c2 Chapter
	if err := json.Unmarshal([]byte(numForm), &c2); err != nil {
		t.Fatalf("数字 dirid 解析失败: %v", err)
	}
	if c2.DirId.String() != "113680" {
		t.Errorf("解析结果不符: %+v", c2)
	}

	t.Logf("两种形态均可解析：%s / %s", c1.DirId.String(), c2.DirId.String())
}

// Book 的 dirid 同理
func TestBookDirIdBothTypes(t *testing.T) {
	for _, raw := range []string{
		`{"id":"4767","dirid":"301","title":"书"}`,
		`{"id":"113680","dirid":113680,"title":"书"}`,
	} {
		var b Book
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			t.Errorf("解析失败 %s: %v", raw, err)
			continue
		}
		t.Logf("id=%s dirid=%s", b.Id, b.DirId.String())
	}
}
