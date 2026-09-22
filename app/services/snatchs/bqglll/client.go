package bqglll

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	xhttp "github.com/vckai/novel/app/librarys/net/http"
	"github.com/vckai/novel/app/utils"
)

/*
bqglll.cc 接口客户端
====================

该站是「前端 SPA + 后端 JSON API」结构，正文不在 HTML 里，
必须走接口获取。接口情况（均为实测）：

	GET /api/book?id=N          书籍元数据，无鉴权
	GET /api/booklist?id=N      目录（返回标题数组），无鉴权
	GET /api/search?q=KW        搜索，无鉴权
	GET /api/sort?sort=xx       分类，无鉴权
	GET /api/chapter?token=..   正文，强制鉴权

设计要点：

  - 域名池：apibi.cc / apiqu.cc / apige.cc 三个域名同后端（实测同 token
    同响应），依次尝试，提高可用性。

  - 复用项目既有的节流器（utils.ThrottleInstance）：
    同域名串行 + 最小间隔，避免对目标站造成冲击，
    这一约定与其它采集器保持一致。

  - 正文接口的 token 由 token.go 生成，密钥为固定常量，
    若站点更换密钥只需改 token.go 中的常量。
*/

// 接口域名池（实测同后端，依次尝试）
var apiHosts = []string{
	"https://apibi.cc",
	"https://apiqu.cc",
	"https://apige.cc",
}

// 请求超时（原为固定值，现由设置项 BqglllTimeout 控制，见 getSettings）
const requestTimeout = 20 * time.Second

// 章节页正文在接口中按固定长度切片返回，单次上限未知；
// 实测最长的一章约 1.2 万字可一次取回，无需分页。

// Client 接口客户端
type Client struct{}

func NewClient() *Client {
	return &Client{}
}

/* ---------- 数据结构 ---------- */

// FlexString 兼容「字符串或数字」的 JSON 字段
//
// 该站部分字段的类型并不稳定，例如 /api/chapter 的 dirid：
// 当 dirid 等于书籍 ID 时返回数字，不同时返回字符串。
// 用一个自定义类型兼容两种情况，避免因类型变化导致解析整体失败。
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" {
		*f = ""
		return nil
	}
	*f = FlexString(s)
	return nil
}

// String 取值
func (f FlexString) String() string { return string(f) }

// Book 书籍元数据（/api/book）
//
// 注意：该接口的所有字段均为字符串（含 id 与 dirid），
// 与 /api/chapter 返回真正的数字类型不同，故此处统一用 string，
// 需要数值时再用 IDToInt 转换。
type Book struct {
	Id            string     `json:"id"`
	Title         string     `json:"title"`
	SortName      string     `json:"sortname"`
	Author        string     `json:"author"`
	Full          string     `json:"full"`
	Intro         string     `json:"intro"`
	LastChapterId string     `json:"lastchapterid"`
	LastChapter   string     `json:"lastchapter"`
	LastUpdate    string     `json:"lastupdate"`
	DirId         FlexString `json:"dirid"`
}

// Chapter 章节正文（/api/chapter）
type Chapter struct {
	Id          int        `json:"id"`
	ChapterId   int        `json:"chapterid"`
	DirId       FlexString `json:"dirid"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	ChapterName string     `json:"chaptername"`
	CS          int        `json:"cs"` // 最大有效 chapterid
	CK          string     `json:"ck"`
	Txt         string     `json:"txt"`
	Time        int64      `json:"time"`
	Md5         string     `json:"md5"`
}

// BookList 目录响应（/api/booklist）
type BookList struct {
	List []string `json:"list"`
}

// SearchResp 搜索响应（/api/search）
type SearchResp struct {
	Data  []Book `json:"data"`
	Title string `json:"title"`
}

/* ---------- 接口方法 ---------- */

// GetBook 取书籍元数据
func (c *Client) GetBook(id string) (*Book, error) {
	raw, err := c.get("/api/book", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}

	var b Book
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("解析书籍信息失败: %w", err)
	}

	if b.Id == "" || b.Title == "" {
		return nil, fmt.Errorf("书籍不存在: id=%s", id)
	}

	return &b, nil
}

// GetBookList 取章节目录（返回章节标题数组，索引自 1 起）
func (c *Client) GetBookList(id string) ([]string, error) {
	raw, err := c.get("/api/booklist", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}

	var bl BookList
	if err := json.Unmarshal(raw, &bl); err != nil {
		return nil, fmt.Errorf("解析目录失败: %w", err)
	}

	return bl.List, nil
}

// GetChapter 取章节正文（需 token）
func (c *Client) GetChapter(id, chapterId int) (*Chapter, error) {
	token, err := EncryptToken(ChapterParams{
		ID:        uint32(id),
		ChapterID: uint32(chapterId),
	})
	if err != nil {
		return nil, err
	}

	// token 已做 URL 编码，直接拼接，避免 url.Values 二次编码
	raw, err := c.getRaw("/api/chapter?token=" + token)
	if err != nil {
		return nil, err
	}

	var ch Chapter
	if err := json.Unmarshal(raw, &ch); err != nil {
		return nil, fmt.Errorf("解析章节失败: %w", err)
	}

	// 越界时接口返回 200 但 chaptername 为 null、txt 为占位文案
	if ch.ChapterName == "" || ch.Txt == "" {
		return nil, fmt.Errorf("章节内容为空: id=%d chapterid=%d", id, chapterId)
	}

	return &ch, nil
}

// Search 搜索（按书名或作者）
func (c *Client) Search(kw string) ([]Book, error) {
	// 注意：中文参数必须 URL 编码，否则站点返回 400
	raw, err := c.get("/api/search", url.Values{"q": {kw}})
	if err != nil {
		return nil, err
	}

	var sr SearchResp
	if err := json.Unmarshal(raw, &sr); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	return sr.Data, nil
}

/* ---------- 内部实现 ---------- */

// get 发起 GET 请求（参数走 query）
func (c *Client) get(path string, q url.Values) ([]byte, error) {
	u := path
	if len(q) > 0 {
		u = path + "?" + q.Encode()
	}
	return c.getRaw(u)
}

// getRaw 依次尝试域名池，带节流
func (c *Client) getRaw(path string) ([]byte, error) {
	var lastErr error

	for _, host := range apiHosts {
		full := host + path

		body, err := c.fetch(full)
		if err == nil {
			return body, nil
		}

		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("接口不可用")
	}

	return nil, lastErr
}

// fetch 单次请求（含节流）
func (c *Client) fetch(rawurl string) ([]byte, error) {
	// 与其它采集器共用节流器：同域名串行 + 最小间隔 + 抖动
	throttle := utils.ThrottleInstance()
	interval, jitter := snatchPace()

	retry := getSettings().Retry
	if retry <= 0 {
		retry = 1 // 至少请求一次
	}

	var lastErr error
	for attempt := 0; attempt < retry; attempt++ {
		throttle.Wait(rawurl, interval, jitter)

		body, err := c.do(rawurl)
		throttle.Done(rawurl)

		if err == nil {
			return body, nil
		}

		lastErr = err

		// 指数退避
		time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
	}

	return nil, lastErr
}

// do 实际发起一次 HTTP 请求
func (c *Client) do(rawurl string) ([]byte, error) {
	to := time.Duration(getSettings().TimeoutSec) * time.Second

	client := xhttp.NewClient(&xhttp.ClientConfig{
		Timeout:   to,
		Dial:      to,
		KeepAlive: 60 * time.Second,
		ProxyURL:  proxyURL(),
	})

	body, _, err := client.Get(context.TODO(), rawurl, nil)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("响应为空")
	}

	return body, nil
}

// snatchPace 取采集节奏（与项目其它采集器一致，可在后台配置）
//
// 通过 SetPaceProvider 注入，避免本包直接依赖 services 造成循环引用。
var paceProvider func() (time.Duration, time.Duration)

// SetPaceProvider 注入节奏提供者（在 services 包初始化时调用）
func SetPaceProvider(f func() (time.Duration, time.Duration)) {
	paceProvider = f
}

func snatchPace() (time.Duration, time.Duration) {
	if paceProvider != nil {
		return paceProvider()
	}

	// 未注入时用设置项里的默认值（该站响应较慢，间隔比通用默认更宽）
	s := getSettings()

	return time.Duration(s.IntervalMs) * time.Millisecond,
		time.Duration(s.JitterMs) * time.Millisecond
}

// proxyURL 取代理地址（同样由外部注入，避免循环依赖）
var proxyProvider func() string

// SetProxyProvider 注入代理提供者
func SetProxyProvider(f func() string) {
	proxyProvider = f
}

func proxyURL() string {
	if proxyProvider != nil {
		return proxyProvider()
	}
	return ""
}

// IDToInt 把站点返回的字符串 ID 转为整数
//
// 接口返回的 id 是字符串（如 "113680"），部分场景需要数值形态。
func IDToInt(s string) int {
	var n int
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n
}

// FetchPage 抓取一个普通网页（非接口）
//
// 用于 SEO 页面：爬虫扫描时只能拿到 SEO 地址，需要从页面里
// 换算接口所用的内部 ID。
func (c *Client) FetchPage(rawurl string) (string, error) {
	body, err := c.fetch(rawurl)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

/* ---------- 采集源设置 ---------- */

/*
Settings 采集源可调参数
======================

由 services 侧从数据库配置读取后注入。放在这里而不是直接读配置的原因：
本包被 services 依赖，直接引用会形成循环，故沿用 provider 注入模式。
*/
type Settings struct {
	// 采集节奏
	IntervalMs int // 请求间隔（毫秒）
	JitterMs   int // 间隔抖动（毫秒）
	TimeoutSec int // 单次请求超时（秒）
	Retry      int // 失败重试次数

	// 内容处理
	MaxDesc     int    // 简介最大长度（字符）
	DefaultCate uint32 // 分类未匹配时的兜底 ID

	// 投毒防护
	PoisonCheck     bool // 是否启用投毒检测
	PoisonThreshold int  // 连续命中多少次暂停
}

// DefaultSettings 默认值（配置缺失时使用）
func DefaultSettings() Settings {
	return Settings{
		IntervalMs:      2500,
		JitterMs:        1200,
		TimeoutSec:      30,
		Retry:           3,
		MaxDesc:         2555,
		DefaultCate:     13,
		PoisonCheck:     true,
		PoisonThreshold: 3,
	}
}

var settingsProvider func() Settings

// SetSettingsProvider 注入设置提供者（在 services 包中调用）
func SetSettingsProvider(f func() Settings) {
	settingsProvider = f
}

// getSettings 取当前设置
func getSettings() Settings {
	if settingsProvider == nil {
		return DefaultSettings()
	}

	s := settingsProvider()

	// 兜底：任一为 0 时回落到默认值，避免配置缺失导致行为异常
	d := DefaultSettings()
	if s.IntervalMs <= 0 {
		s.IntervalMs = d.IntervalMs
	}
	if s.TimeoutSec <= 0 {
		s.TimeoutSec = d.TimeoutSec
	}
	if s.Retry < 0 {
		s.Retry = d.Retry
	}
	if s.MaxDesc <= 0 || s.MaxDesc > 2555 {
		s.MaxDesc = d.MaxDesc
	}
	if s.DefaultCate == 0 {
		s.DefaultCate = d.DefaultCate
	}
	if s.PoisonThreshold <= 0 {
		s.PoisonThreshold = d.PoisonThreshold
	}

	return s
}

// CurrentSettings 取当前生效的设置（供外部查询）
func CurrentSettings() Settings {
	return getSettings()
}

/* ---------- 分类页接口 ---------- */

// SortItem /json 接口返回的单个条目
type SortItem struct {
	URLList     string `json:"url_list"`    // SEO 页面地址
	URLImg      string `json:"url_img"`     // 封面图（含内部 ID）
	ArticleName string `json:"articlename"` // 书名（站点用无下划线写法）
	Author      string `json:"author"`      // 作者
	Intro       string `json:"intro"`       // 简介
}

// GetSortPage 取分类页的一页数据
//
// 该站的分类页服务端只渲染 15 本，更多内容由前端 JS 调本接口加载。
// 因此要批量发现书籍，必须走这里而不是解析 HTML。
//
// 返回空数组表示已到末页（实测约 20-30 页）。
func (c *Client) GetSortPage(sortID, page int) ([]SortItem, error) {
	if sortID <= 0 || page <= 0 {
		return nil, fmt.Errorf("参数错误: sortid=%d page=%d", sortID, page)
	}

	path := fmt.Sprintf("/json?sortid=%d&page=%d", sortID, page)

	raw, err := c.fetchByPage(path)
	if err != nil {
		return nil, err
	}

	var items []SortItem
	if err := json.Unmarshal(raw, &items); err != nil {
		// 末页可能返回非 JSON（如空响应），按空处理而非报错
		return nil, nil
	}

	return items, nil
}

// fetchByPage 抓取分类页接口
//
// 与接口请求的区别：这里请求的是站点自身页面（非 API 域名），
// 故直接按站点地址请求，不再走 API 域名池。
func (c *Client) fetchByPage(path string) ([]byte, error) {
	return c.fetch(pageHost + path)
}

// pageHost 分类页接口所在域名
const pageHost = "https://www.bqglll.cc"
