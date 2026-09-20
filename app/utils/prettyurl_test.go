package utils

import "testing"

// 伪静态地址改写
func TestPrettyURL(t *testing.T) {
	cases := []struct {
		endpoint string
		url      string
		want     string
	}{
		// 小说详情：/book/index?id=1 -> /book/1.html
		{"home.BookController.Index", "/book/index?id=1", "/book/1.html"},
		{"home.BookController.Index", "/book/index?id=12345", "/book/12345.html"},

		// 章节阅读：/book/detail?novid=1&id=2 -> /book/1/2.html
		{"home.BookController.Detail", "/book/detail?novid=1&id=2", "/book/1/2.html"},
		// 参数顺序颠倒也应正确
		{"home.BookController.Detail", "/book/detail?id=2&novid=1", "/book/1/2.html"},

		// 不相关的方法不改写
		{"home.BookController.AjaxRank", "/book/ajaxrank", "/book/ajaxrank"},
		{"home.HomeController.Index", "/home/index", "/home/index"},

		// 移动端小说详情：/m/book/index?id=1 -> /m/book/1.html
		{"m.BookController.Index", "/m/book/index?id=1", "/m/book/1.html"},
		// 移动端章节阅读
		{"m.BookController.Detail", "/m/book/detail?novid=2&id=30", "/m/book/2/30.html"},

		// 缺少必要参数时保持原样
		{"home.BookController.Detail", "/book/detail?novid=1", "/book/detail?novid=1"},
		{"home.BookController.Index", "/book/index", "/book/index"},
	}

	for _, c := range cases {
		if got := PrettyURL(c.endpoint, c.url); got != c.want {
			t.Errorf("PrettyURL(%q,%q) = %q, 期望 %q", c.endpoint, c.url, got, c.want)
		}
	}
}

// 分类页伪静态地址生成
func TestPrettyCateURL(t *testing.T) {
	cases := []struct {
		prefix                                string
		cateId, page, status, textNum, up, ot int
		want                                  string
	}{
		// 第 1 页、无筛选 -> 最简
		{"", 1, 1, 0, 0, 0, 1, "/cate/1.html"},
		// 仅翻页
		{"", 1, 2, 0, 0, 0, 1, "/cate/1/p2.html"},
		{"", 11, 5, 0, 0, 0, 1, "/cate/11/p5.html"},
		// 带筛选 -> 5 段
		{"", 1, 2, 1, 0, 0, 1, "/cate/1/p2/1_0_0_1.html"},
		{"", 1, 1, 0, 3, 2, 3, "/cate/1/p1/0_3_2_3.html"},
		// 移动端前缀
		{"/m", 1, 1, 0, 0, 0, 1, "/m/cate/1.html"},
		{"/m", 1, 2, 0, 0, 0, 1, "/m/cate/1/p2.html"},
		// 无分类
		{"", 0, 1, 0, 0, 0, 1, "/cate/"},
	}

	for _, c := range cases {
		got := PrettyCateURL(c.prefix, c.cateId, c.page, c.status, c.textNum, c.up, c.ot)
		if got != c.want {
			t.Errorf("PrettyCateURL(%q,%d,%d,%d,%d,%d,%d) = %q, 期望 %q",
				c.prefix, c.cateId, c.page, c.status, c.textNum, c.up, c.ot, got, c.want)
		}
	}
}

// 由旧式分类地址改写
func TestPrettyURLCate(t *testing.T) {
	cases := []struct{ endpoint, url, want string }{
		{"home.HomeController.Cate", "/home/cate?id=1", "/cate/1.html"},
		{"home.HomeController.Cate", "/home/cate?id=1&p=2", "/cate/1/p2.html"},
		{"home.HomeController.Cate", "/home/cate?id=1&status=1&ot=1", "/cate/1/p1/1_0_0_1.html"},
		{"m.BookController.List", "/m/book/list?cate_id=1", "/m/cate/1.html"},
		// 未指定分类时保持原地址（避免生成无效的 /cate/）
		{"home.HomeController.Cate", "/home/cate", "/home/cate"},
	}
	for _, c := range cases {
		if got := PrettyURL(c.endpoint, c.url); got != c.want {
			t.Errorf("PrettyURL(%q,%q) = %q, 期望 %q", c.endpoint, c.url, got, c.want)
		}
	}
}
