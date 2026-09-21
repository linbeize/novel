package api

import (
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services"
)

// 初始化应用依赖（数据库、服务），供接口测试使用。
// 需在项目根目录运行，以便读取 conf/ 下的配置。
var apiInitOnce sync.Once

func initAPIApp() {
	apiInitOnce.Do(func() {
		if wd, err := os.Getwd(); err == nil && strings.HasSuffix(wd, "/app/controllers/api") {
			_ = os.Chdir("../../..")
		}
		beego.BConfig.RunMode = "prod"
		beego.LoadAppConfig("ini", "conf/app.conf")
		models.InitDB()
		services.Init()
	})
}

// 构造控制器实例并执行指定方法，返回响应内容
//
// 注意：控制器在返回前会调用 StopRun()，其实现是 panic(beego.ErrAbort)，
// 由 beego 的 HTTP 层负责 recover。测试里没有这一层，故需自行捕获，
// 否则测试会直接失败。
func callAPI(t *testing.T, method string, fn func(*NovelController), query string) string {
	t.Helper()
	initAPIApp()

	rec := httptest.NewRecorder()
	ctx := context.NewContext()
	ctx.Reset(rec, httptest.NewRequest("GET", "/api?"+query, nil))

	c := &NovelController{}
	c.Ctx = ctx
	c.Data = make(map[interface{}]interface{})
	c.Prepare()

	runHandler(fn, c)

	return rec.Body.String()
}

// 执行 handler 并忽略 StopRun 的 panic
func runHandler(fn func(*NovelController), c *NovelController) {
	defer func() {
		if r := recover(); r != nil {
			if r == beego.ErrAbort {
				return
			}
			panic(r)
		}
	}()
	fn(c)
}

// 首页聚合应返回各组数据
func TestAPIHome(t *testing.T) {
	body := callAPI(t, "GET", func(c *NovelController) { c.Home() }, "")

	for _, key := range []string{"banners", "recs", "hots", "new_ups", "ranks", "theme"} {
		if !strings.Contains(body, `"`+key+`"`) {
			t.Errorf("首页返回缺少字段 %q", key)
		}
	}
	if !strings.Contains(body, `"ret":0`) {
		t.Errorf("ret 应为 0：%s", trunc(body, 200))
	}
}

// 分类列表
func TestAPICates(t *testing.T) {
	body := callAPI(t, "GET", func(c *NovelController) { c.Cates() }, "")

	if !strings.Contains(body, `"ret":0`) {
		t.Errorf("ret 应为 0：%s", trunc(body, 200))
	}
	if !strings.Contains(body, `"total"`) {
		t.Errorf("缺少 total 字段")
	}
}

// 书籍列表：分页参数边界
func TestAPIBooksPaging(t *testing.T) {
	// 正常分页
	body := callAPI(t, "GET", func(c *NovelController) { c.Books() }, "size=2&p=1")
	if !strings.Contains(body, `"size":2`) {
		t.Errorf("size 应为 2：%s", trunc(body, 200))
	}

	// 超过上限应被限制为 100
	body = callAPI(t, "GET", func(c *NovelController) { c.Books() }, "size=9999")
	if !strings.Contains(body, `"size":100`) {
		t.Errorf("size 应被限制为 100：%s", trunc(body, 200))
	}

	// 非法页码应回退到 1
	body = callAPI(t, "GET", func(c *NovelController) { c.Books() }, "p=-5")
	if !strings.Contains(body, `"page":1`) {
		t.Errorf("页码应回退为 1：%s", trunc(body, 200))
	}
}

// 书籍详情：存在与不存在
func TestAPIDetail(t *testing.T) {
	// 取一个确实存在的小说
	novs := services.NovelService.GetNewUps(1, 0)
	if len(novs) == 0 {
		t.Skip("库中没有小说，跳过")
	}
	id := novs[0].Id

	// 路径参数在真实请求里由路由填充，这里用 SetParam 模拟
	body := callAPIParam(t, func(c *NovelController) { c.Detail() }, ":id", id)
	if !strings.Contains(body, `"ret":0`) {
		t.Errorf("存在的书籍应返回 ret=0：%s", trunc(body, 200))
	}
	if !strings.Contains(body, `"cover":"http`) {
		t.Errorf("封面应为绝对地址：%s", trunc(body, 400))
	}

	// 不存在的书籍
	body = callAPIParam(t, func(c *NovelController) { c.Detail() }, ":id", uint32(99999999))
	if !strings.Contains(body, `"ret":1001`) {
		t.Errorf("不存在的书籍应返回 1001：%s", trunc(body, 200))
	}
}

// 章节目录
func TestAPIChapters(t *testing.T) {
	// 通过分类列表取有章节的书（GetNewUps 只覆盖最近更新，可能都是刚采的零章节书）
	var target uint32
	novs, _ := services.NovelService.GetList(50, 0, map[string]interface{}{"count": false, "ot": 1})
	for _, n := range novs {
		if n.ChapterNum > 0 {
			target = n.Id
			break
		}
	}
	if target == 0 {
		t.Skip("库中没有含章节的小说，跳过")
	}

	body := callAPIParam(t, func(c *NovelController) { c.Chapters() }, ":id", target)
	if !strings.Contains(body, `"ret":0`) {
		t.Fatalf("目录接口失败：%s", trunc(body, 200))
	}
	if !strings.Contains(body, `"list"`) {
		t.Errorf("缺少 list 字段")
	}
}

// 章节正文：缺少 novid 应报参数错误
func TestAPIChapterParam(t *testing.T) {
	body := callAPIParam(t, func(c *NovelController) { c.Chapter() }, ":id", uint64(1))
	if !strings.Contains(body, `"ret":1`) {
		t.Errorf("缺少 novid 应返回参数错误：%s", trunc(body, 200))
	}
}

// 封面地址转换
func TestAbsURL(t *testing.T) {
	initAPIApp()

	rec := httptest.NewRecorder()
	ctx := context.NewContext()
	ctx.Reset(rec, httptest.NewRequest("GET", "/api/home", nil))

	c := &NovelController{}
	c.Ctx = ctx
	c.Data = make(map[interface{}]interface{})

	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		// 已是绝对地址
		{"https://cdn.example.com/a.jpg", "https://cdn.example.com/a.jpg"},
		{"//cdn.example.com/a.jpg", "//cdn.example.com/a.jpg"},
	}

	for _, x := range cases {
		if got := c.absURL(x.in); got != x.want {
			t.Errorf("absURL(%q) = %q，期望 %q", x.in, got, x.want)
		}
	}

	// 相对路径应补成绝对地址
	got := c.absURL("/public/up/a.jpg")
	if !strings.HasPrefix(got, "http") || !strings.HasSuffix(got, "/public/up/a.jpg") {
		t.Errorf("相对路径未正确补全：%q", got)
	}
	// 不应出现双斜杠
	if strings.Contains(got, "//public") {
		t.Errorf("拼接出现双斜杠：%q", got)
	}
}

// 搜索关键词的高亮标签应被去掉
func TestStripHighlight(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"诡秘之主", "诡秘之主"},
		{`<font color="red">诡秘</font>之主`, "诡秘之主"},
		{`<font color="red">a</font><font color="red">b</font>`, "ab"},
	}

	for _, c := range cases {
		if got := stripHighlight(c.in); got != c.want {
			t.Errorf("stripHighlight(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

// 带路由参数的调用
func callAPIParam(t *testing.T, fn func(*NovelController), key string, val interface{}) string {
	t.Helper()
	initAPIApp()

	rec := httptest.NewRecorder()
	ctx := context.NewContext()
	ctx.Reset(rec, httptest.NewRequest("GET", "/api", nil))
	ctx.Input.SetParam(key, toStr(val))

	c := &NovelController{}
	c.Ctx = ctx
	c.Data = make(map[interface{}]interface{})
	c.Prepare()

	runHandler(fn, c)

	return rec.Body.String()
}

func toStr(v interface{}) string {
	switch x := v.(type) {
	case uint32:
		return itoa(uint64(x))
	case uint64:
		return itoa(x)
	case int:
		return itoa(uint64(x))
	case string:
		return x
	}
	return ""
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
