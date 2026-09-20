package models

import (
	"encoding/json"
	"testing"
)

// 新增的目录分页字段必须能正确序列化/反序列化，
// 否则规则存库后翻页配置会丢失。
func TestSnatchRulePageFieldsRoundTrip(t *testing.T) {
	r := NewSnatchRule()
	r.Rules.ChapterCatalogSelector = ".book_list2 a"
	r.Rules.ChapterPageURLTemplate = "https://www.5566xs.com{book}index_{page}.html"
	r.Rules.ChapterPageStart = 2
	r.Rules.ChapterPageMax = 200
	r.Rules.InfoNextPageSelector = "a#next1"
	r.Rules.InfoDescSelector = "article.font_max"

	// 结构体 -> JSON 字符串（入库）
	if err := r.Encode(); err != nil {
		t.Fatal("Encode 失败:", err)
	}

	if len(r.Rule) == 0 {
		t.Fatal("Encode 后 Rule 为空")
	}

	// 确认新字段确实写进了 JSON
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(r.Rule), &raw); err != nil {
		t.Fatal("Rule 不是合法 JSON:", err)
	}
	for _, k := range []string{"chapter_page_url_template", "chapter_page_start", "chapter_page_max"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("JSON 中缺少字段 %s", k)
		}
	}

	// 模拟从数据库读回
	loaded := NewSnatchRule()
	loaded.Rule = r.Rule
	if err := loaded.Decode(); err != nil {
		t.Fatal("Decode 失败:", err)
	}

	if loaded.Rules.ChapterPageURLTemplate != r.Rules.ChapterPageURLTemplate {
		t.Errorf("模板字段丢失: %q", loaded.Rules.ChapterPageURLTemplate)
	}
	if loaded.Rules.ChapterPageStart != 2 {
		t.Errorf("起始页丢失: %d", loaded.Rules.ChapterPageStart)
	}
	if loaded.Rules.ChapterPageMax != 200 {
		t.Errorf("最大页丢失: %d", loaded.Rules.ChapterPageMax)
	}
	if loaded.Rules.InfoNextPageSelector != "a#next1" {
		t.Errorf("下一页选择器丢失: %q", loaded.Rules.InfoNextPageSelector)
	}
}

// 规则 JSON 长度不应超过数据库字段限制 size(2555)
func TestSnatchRuleWithinColumnLimit(t *testing.T) {
	r := NewSnatchRule()
	r.Rules.ChapterPageURLTemplate = "https://www.5566xs.com{book}index_{page}.html"
	r.Rules.ChapterCatalogSelector = ".book_list2 a"
	r.Rules.InfoDescSelector = "article.font_max"

	if err := r.Encode(); err != nil {
		t.Fatal(err)
	}

	if len(r.Rule) > 2555 {
		t.Errorf("Rule 长度 %d 超过字段限制 2555", len(r.Rule))
	}
}
