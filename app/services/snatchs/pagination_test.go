package snatchs

import (
	"testing"
)

// 章节正文分页：URL 归一化
func TestChapterBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/0/106/122701.html", "/0/106/122701.html"},
		{"/0/106/122701_2.html", "/0/106/122701.html"},
		{"/0/106/122701_3.html", "/0/106/122701.html"},
		{"/0/106/122702.html", "/0/106/122702.html"},
		{"https://x.com/0/106/122701_2.html", "https://x.com/0/106/122701.html"},
	}

	for _, c := range cases {
		if got := chapterBaseURL(c.in); got != c.want {
			t.Errorf("chapterBaseURL(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

// 章节正文分页：同章判定
// 关键场景：末页的「下一章」指向下一章，必须判定为不同章以终止拼接
func TestSameChapterURL(t *testing.T) {
	cases := []struct {
		cur, next string
		want      bool
	}{
		{"/0/106/122701.html", "/0/106/122701_2.html", true},
		{"/0/106/122701_2.html", "/0/106/122701_3.html", true},
		{"/0/106/122701_3.html", "/0/106/122702.html", false},
		{"/0/106/122701.html", "/0/106/122701.html", true},
	}

	for _, c := range cases {
		if got := sameChapterURL(c.cur, c.next); got != c.want {
			t.Errorf("sameChapterURL(%q,%q) = %v, 期望 %v", c.cur, c.next, got, c.want)
		}
	}
}

// 分页标记剔除，如「第(1/3)页」
func TestReChapterPageMark(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<br>第(1/3)页<br>正文", "<br><br>正文"},
		{"第( 2 / 3 )页 正文", " 正文"},
		{"无标记正文", "无标记正文"},
		{"第(10/10)页", ""},
	}

	for _, c := range cases {
		if got := reChapterPageMark.ReplaceAllString(c.in, ""); got != c.want {
			t.Errorf("剔除分页标记(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

// 目录分页：由书籍 URL 推导 {book} 占位符
func TestBookDirPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.5566xs.com/0/106/", "/0/106/"},
		{"https://www.5566xs.com/0/106", "/0/106/"},
		{"https://www.5566xs.com/0/106/index.html", "/0/106/"},
		{"https://www.5566xs.com/135/135251/", "/135/135251/"},
	}

	for _, c := range cases {
		if got := bookDirPath(c.in); got != c.want {
			t.Errorf("bookDirPath(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}
