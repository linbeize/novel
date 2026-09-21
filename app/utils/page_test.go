package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 构造一个分页器：per 为每页条数，nums 为总条数，rawQuery 为查询串
//
// 注意 rawQuery 为空时不能拼出带尾问号的 URL（如 "/book/805.html?"），
// 否则 Go 的 url 包会保留该问号，使 PageLink 结果与真实请求不一致。
func newTestPaginator(per int, nums int64, rawQuery string) *Paginator {
	target := "/book/805.html"
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	return NewPaginator(req, per, nums)
}

// 默认行为：从 query 的 p 参数取当前页
func TestPaginatorPageFromQuery(t *testing.T) {
	cases := []struct {
		query string
		want  int
	}{
		{"", 1},
		{"p=1", 1},
		{"p=3", 3},
		{"p=0", 1},   // 非法，回退第 1 页
		{"p=-5", 1},  // 负数，回退第 1 页
		{"p=abc", 1}, // 非数字，回退第 1 页
		{"p=999", 20},
	}

	for _, c := range cases {
		p := newTestPaginator(100, 1966, c.query) // 20 页
		if got := p.Page(); got != c.want {
			t.Errorf("query=%q: Page() = %d, 期望 %d", c.query, got, c.want)
		}
	}
}

// SetPage 用于伪静态分页：页码在路径中，query 里没有 p
//
// 这是修复的核心——不调用 SetPage 时分页器会始终认为停在第 1 页，
// 导致高亮错误、HasPrev/HasNext 判断错误。
func TestPaginatorSetPageForPrettyURL(t *testing.T) {
	const total = 1966
	const per = 100 // 20 页

	for _, page := range []int{1, 2, 3, 20} {
		// 伪静态地址 /book/805/p3.html 的 query 为空
		p := newTestPaginator(per, total, "")
		p.SetPage(page)

		if got := p.Page(); got != page {
			t.Errorf("SetPage(%d): Page() = %d", page, got)
		}

		// 未设置时必须是第 1 页（说明 SetPage 确实起作用，而非巧合）
		plain := newTestPaginator(per, total, "")
		if plain.Page() != 1 {
			t.Errorf("未调用 SetPage 时应为 1，实际 %d", plain.Page())
		}
	}
}

// SetPage 的越界与非法值收敛
func TestPaginatorSetPageClamp(t *testing.T) {
	const total = 1966
	const per = 100 // 20 页

	cases := []struct {
		set  int
		want int
	}{
		{0, 1},    // 小于 1 -> 第 1 页
		{-3, 1},   // 负数 -> 第 1 页
		{1, 1},    // 正常
		{20, 20},  // 最后一页
		{21, 20},  // 超出 -> 收敛到最后一页
		{999, 20}, // 远超 -> 收敛到最后一页
	}

	for _, c := range cases {
		p := newTestPaginator(per, total, "")
		p.SetPage(c.set)
		if got := p.Page(); got != c.want {
			t.Errorf("SetPage(%d): Page() = %d, 期望 %d", c.set, got, c.want)
		}
	}
}

// 边界：总数为 0 时 Page() 不应为 0
//
// 说明：PageNums() 在总数为 0 时按 ceil(0/per) 得到 0，这是既有行为；
// 模板以 {{if gt .Paginator.PageNums 1}} 判断，为 0 时隐藏分页器，
// 因此不影响页面。此处只断言 Page() 会被收敛到 1，不会出现第 0 页。
func TestPaginatorSetPageEmptySet(t *testing.T) {
	p := newTestPaginator(100, 0, "")
	p.SetPage(5)

	if got := p.Page(); got != 1 {
		t.Errorf("总数为 0 时 Page() = %d, 期望 1", got)
	}
}

// 前/后页判断：伪静态下 SetPage 后必须正确
func TestPaginatorHasPrevNextAfterSetPage(t *testing.T) {
	const total = 1966
	const per = 100 // 20 页

	// 第 1 页：无上一页，有下一页
	p1 := newTestPaginator(per, total, "")
	p1.SetPage(1)
	if p1.HasPrev() {
		t.Error("第 1 页不应有上一页")
	}
	if !p1.HasNext() {
		t.Error("第 1 页应有下一页")
	}

	// 第 10 页：前后都有
	p10 := newTestPaginator(per, total, "")
	p10.SetPage(10)
	if !p10.HasPrev() {
		t.Error("第 10 页应有上一页")
	}
	if !p10.HasNext() {
		t.Error("第 10 页应有下一页")
	}

	// 最后一页：有上一页，无下一页
	plast := newTestPaginator(per, total, "")
	plast.SetPage(20)
	if !plast.HasPrev() {
		t.Error("最后一页应有上一页")
	}
	if plast.HasNext() {
		t.Error("最后一页不应有下一页")
	}
}

// URLBuilder 注入后，分页链接应走自定义生成器
func TestPaginatorURLBuilder(t *testing.T) {
	p := newTestPaginator(100, 1966, "")
	p.SetPage(3)
	p.URLBuilder = func(page int) string {
		return PrettyBookCatalogURL("", 805, page)
	}

	if got := p.PageLink(1); got != "/book/805.html" {
		t.Errorf("第 1 页链接 = %q", got)
	}
	if got := p.PageLink(3); got != "/book/805/p3.html" {
		t.Errorf("第 3 页链接 = %q", got)
	}
	if got := p.PageLinkNext(); got != "/book/805/p4.html" {
		t.Errorf("下一页链接 = %q", got)
	}
	if got := p.PageLinkPrev(); got != "/book/805/p2.html" {
		t.Errorf("上一页链接 = %q", got)
	}
}

// 未注入 URLBuilder 时保持原行为（基于 query 修改 p）
func TestPaginatorURLBuilderNilKeepsOldBehavior(t *testing.T) {
	p := newTestPaginator(100, 1966, "")
	p.SetPage(2)

	if got := p.PageLink(1); got != "/book/805.html" {
		t.Errorf("第 1 页链接 = %q（原行为应去掉 p 参数）", got)
	}
	if got := p.PageLink(4); got != "/book/805.html?p=4" {
		t.Errorf("第 4 页链接 = %q", got)
	}
}
