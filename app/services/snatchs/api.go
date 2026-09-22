package snatchs

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services/snatchs/bqglll"
	"github.com/vckai/novel/app/utils/log"
)

/*
API 型采集器（bqglll.cc）
========================

背景
----
项目原有的采集器是「CSS 选择器 + HTML 解析」，适用于服务端渲染的站点。
而 bqglll.cc 是 SPA + JSON 接口结构：

  - 正文不在 HTML 里，必须调 /api/chapter 获取；
  - 该接口要求 AES 加密的 token，CSS 规则无法表达；
  - 元数据、目录、搜索也走 JSON 接口。

因此无法用现有的规则字段描述，需单独的采集实现。

接入方式
--------
规则表新增 `api_type` 字段（见 models.Rule）。当该字段非空时，
本文件的 ApiSnatch 接管，按接口协议采集；否则仍走原有的
CSS 采集逻辑。这样两种采集器共存，互不影响。

域名与 ID
---------
该站存在双轨 ID：
  - 内部 ID（如 113680）：接口与封面图使用
  - SEO ID（如 104952）：静态页 URL 使用

采集点统一记录内部 ID，形如 `bqglll://113680`，避免与 SEO ID 混淆。
*/

// ApiSnatch API 型采集器
type ApiSnatch struct {
	client *bqglll.Client
}

func NewApiSnatch() *ApiSnatch {
	return &ApiSnatch{client: bqglll.NewClient()}
}

// IsAPI 判断该规则是否走 API 采集
func IsAPI(provider *models.SnatchRule) bool {
	if provider == nil || provider.Rules == nil {
		return false
	}
	return strings.TrimSpace(provider.Rules.APIType) != ""
}

/* ---------- URL 约定 ---------- */

// API 采集点的地址形如 bqglll://113680
const apiScheme = "bqglll://"

// 站点主域（SEO 页面、封面图均在此域名下）
const providerHost = "https://www.bqglll.cc"

// buildLink 由书籍 ID 生成采集点地址
func buildLink(id string) string {
	return apiScheme + id
}

// parseLink 从采集点地址解析书籍 ID
//
// 兼容三种写法，便于手工录入：
//
//	bqglll://113680
//	113680
//	https://www.bqglll.cc/look/104952/   （SEO 页，需先用 API 搜索换到内部 ID）
func parseLink(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("采集点为空")
	}

	if strings.HasPrefix(s, apiScheme) {
		id := strings.TrimPrefix(s, apiScheme)
		if id == "" {
			return "", fmt.Errorf("采集点缺少书籍 ID")
		}
		return id, nil
	}

	// 纯数字
	if isDigits(s) {
		return s, nil
	}

	// SEO 页地址：/look/{seoId}/
	//
	// 该站有双轨 ID：页面 URL 用 SEO ID，接口用内部 ID，两者不同。
	// 爬虫扫描时只能看到 SEO 地址，因此需要据此换算出内部 ID
	// （见 resolveSEOLink）。换算要发一次请求，故在上层做。
	if strings.Contains(s, "/look/") {
		return "", errNeedResolveSEO
	}

	return "", fmt.Errorf("无法识别的采集点: %s", s)
}

// errNeedResolveSEO 表示该地址是 SEO 页面，需先换算为内部 ID
var errNeedResolveSEO = fmt.Errorf("需要换算 SEO 地址")

// bookID 取采集点对应的内部书籍 ID
//
// 兼容两种写法：
//   - bqglll://113680 或 113680  —— 已是内部 ID，直接使用
//   - https://www.bqglll.cc/look/104952/  —— SEO 地址，需换算
//
// SEO 地址这一支是爬虫场景必需的：爬虫在站点里只能看到页面地址。
func (this *ApiSnatch) bookID(rawurl string) (string, error) {
	id, err := parseLink(rawurl)

	if err == errNeedResolveSEO {
		return this.resolveSEOLink(rawurl)
	}

	return id, err
}

// seoIDRe 从 SEO 页面地址中取出 SEO ID
var seoIDRe = regexp.MustCompile(`/look/(\d+)`)

// extractSEOPageID 从 SEO 页面地址解析出 SEO ID
func extractSEOPageID(raw string) (string, bool) {
	m := seoIDRe.FindStringSubmatch(raw)
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

// resolveSEOLink 把 SEO 页面地址换算成内部 ID
//
// 依据：SEO 页面里的封面图路径形如 bookimg/{floor(id/1000)}/{id}.jpg，
// 其中的 id 即接口所需的内部 ID。实测多本书均一致。
//
// 之所以能可靠取到：封面图是服务端直接渲染在 HTML 里的，
// 不依赖 JS；且该路径是站点自己的图片托管规则，不会随内容变化。
func (this *ApiSnatch) resolveSEOLink(raw string) (string, error) {
	seoID, ok := extractSEOPageID(raw)
	if !ok {
		return "", fmt.Errorf("无法从地址中解析 SEO ID: %s", raw)
	}

	pageURL := providerHost + "/look/" + seoID + "/"
	body, err := this.client.FetchPage(pageURL)
	if err != nil {
		return "", fmt.Errorf("获取 SEO 页面失败: %w", err)
	}

	m := coverIDRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return "", fmt.Errorf("SEO 页面中未找到内部 ID: %s", pageURL)
	}

	innerID := m[1]
	log.Debug(fmt.Sprintf("SEO 地址换算: %s → 内部ID=%s", raw, innerID))

	return innerID, nil
}

// coverIDRe 从封面图路径中取内部 ID
var coverIDRe = regexp.MustCompile(`bookimg/\d+/(\d+)\.jpg`)

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

/* ---------- 采集接口 ---------- */

// FindNovel 搜索小说
func (this *ApiSnatch) FindNovel(provider *models.SnatchRule, kw string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	books, err := this.client.Search(kw)
	if err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return nil, fmt.Errorf("未搜索到小说: %s", kw)
	}

	// 取第一条（与其它采集器行为一致）
	b := books[0]

	nov := models.NewNovel()
	nov.Name = b.Title
	nov.Author = b.Author
	nov.Desc = clipDesc(b.Intro)

	// 搜索接口只返回 id/title/author/intro，缺少分类、状态、最新章节，
	// 故再取一次详情补齐。失败不影响搜索结果的返回。
	if detail, derr := this.client.GetBook(b.Id); derr == nil {
		nov.CateName = detail.SortName
		nov.CateId = mapCateId(provider, detail.SortName)
		nov.ChapterTitle = detail.LastChapter
		if detail.Full == "完结" || detail.Full == "完本" {
			nov.Status = models.BOOKFINISH
		} else {
			nov.Status = models.BOOKOPEN
		}
	} else {
		log.Warn("获取书籍详情失败（搜索结果仍可用）：", b.Id, " ", derr)
	}

	// 封面：bookimg/{floor(id/1000)}/{id}.jpg
	nov.Cover = coverURL(b.Id)

	return &SnatchInfo{
		Nov:        nov,
		Url:        buildLink(b.Id),
		Source:     provider.Code,
		ChapterUrl: buildLink(b.Id),
	}, nil
}

// GetNovel 取书籍详情
func (this *ApiSnatch) GetNovel(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	id, err := this.bookID(rawurl)
	if err != nil {
		return nil, err
	}

	b, err := this.client.GetBook(id)
	if err != nil {
		return nil, err
	}

	nov := models.NewNovel()
	nov.Name = b.Title
	nov.Author = b.Author
	nov.CateName = b.SortName
	nov.CateId = mapCateId(provider, b.SortName)
	nov.Desc = clipDesc(b.Intro)
	nov.ChapterTitle = b.LastChapter
	nov.Cover = coverURL(b.Id)

	// 完本状态
	if b.Full == "完结" || b.Full == "完本" {
		nov.Status = models.BOOKFINISH
	} else {
		nov.Status = models.BOOKOPEN
	}

	return &SnatchInfo{
		Nov:        nov,
		Url:        buildLink(b.Id),
		Source:     provider.Code,
		ChapterUrl: buildLink(b.Id),
	}, nil
}

// GetChapters 取章节目录
//
// 接口返回章节标题数组，索引自 1 起（chapterid 即数组下标）。
// 这里统一编号为 1..N，与其它采集器的行为保持一致。
func (this *ApiSnatch) GetChapters(provider *models.SnatchRule, rawurl string) ([]*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	id, err := this.bookID(rawurl)
	if err != nil {
		return nil, err
	}

	t1 := time.Now()

	// 目录必须用 dirid（目录 ID），而非书籍 ID。
	//
	// 该站接口存在一个不一致：/api/book 同时返回 id 与 dirid，
	// 而 /api/booklist 的 id 参数实际取的是 dirid。
	// 多数书的 dirid 恰好等于 id，因此这个差异容易漏掉；
	// 但抽样统计约三分之一的书籍两者不同，用错就拿不到目录
	// （接口返回 {"list":null}）。
	// 正文接口 /api/chapter 则两个 ID 都能用，故此处仅目录需要换 ID。
	dirId := id
	if b, derr := this.client.GetBook(id); derr == nil {
		if d := strings.TrimSpace(b.DirId.String()); d != "" && d != id {
			dirId = d
			log.Debug(fmt.Sprintf("[%s]目录 ID 与书籍 ID 不同: book=%s dir=%s",
				provider.Name, id, dirId))
		}
	} else {
		log.Warn("获取 dirid 失败，按书籍 ID 尝试目录：", id, " ", derr)
	}

	titles, err := this.client.GetBookList(dirId)
	if err != nil {
		return nil, err
	}

	// 兼容接口返回 {"list":null} 的情况：换个 ID 再试一次
	if len(titles) == 0 && dirId != id {
		log.Warn("目录为空，回退用书籍 ID 重试：", id)
		titles, _ = this.client.GetBookList(id)
	}
	if len(titles) == 0 {
		return nil, fmt.Errorf("目录为空: id=%s dirid=%s", id, dirId)
	}

	links := make([]*SnatchInfo, 0, len(titles))
	for i, title := range titles {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}

		chap := models.NewChapter()
		chap.Title = title
		chap.ChapterNo = uint32(i + 1)
		// 章节地址：id + 序号，供 GetChapter 解析
		chap.Link = fmt.Sprintf("%s%s/%d", apiScheme, id, i+1)

		links = append(links, &SnatchInfo{
			Chap:    chap,
			UseTime: time.Since(t1),
		})
	}

	log.Debug(fmt.Sprintf("[%s]获取小说章节列表[%s]，共 %d 章，使用时间：%v",
		provider.Name, rawurl, len(links), time.Since(t1)))

	return links, nil
}

// GetChapter 取章节正文
//
// 地址形如 bqglll://113680/688，第二段为章节序号。
func (this *ApiSnatch) GetChapter(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	if provider == nil {
		return nil, ErrNotProvider
	}

	bookId, chapNo, err := parseChapterLink(rawurl)
	if err != nil {
		return nil, err
	}

	ch, err := this.client.GetChapter(bookId, chapNo)
	if err != nil {
		return nil, err
	}

	chap := models.NewChapter()
	chap.Title = strings.TrimSpace(ch.ChapterName)
	chap.NovId = uint32(bookId)
	chap.ChapterNo = uint32(chapNo)
	// 正文按 \n 分段，这里转为 HTML 片段以与其它采集器一致
	// （入库格式统一为含 <br/> 的片段，前台模板按 HTML 渲染）
	chap.Desc = textToHTML(ch.Txt)
	chap.Link = rawurl
	chap.Source = provider.Code
	chap.TextNum = uint32(len([]rune(ch.Txt)))

	return &SnatchInfo{
		Chap:   chap,
		Url:    rawurl,
		Source: provider.Code,
	}, nil
}

// GetChapterFull 取完整章节正文
//
// 该站的正文接口一次返回整章（前端的分页是纯客户端行为），
// 因此无需像 HTML 采集那样再拼接分页，直接复用 GetChapter 即可。
func (this *ApiSnatch) GetChapterFull(provider *models.SnatchRule, rawurl string) (*SnatchInfo, error) {
	return this.GetChapter(provider, rawurl)
}

/* ---------- 内部辅助 ---------- */

// parseChapterLink 解析章节采集点，返回书籍 ID 与章节序号
func parseChapterLink(rawurl string) (int, int, error) {
	s := strings.TrimSpace(rawurl)
	s = strings.TrimPrefix(s, apiScheme)

	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("章节地址格式错误: %s", rawurl)
	}

	bookId := bqglll.IDToInt(parts[0])
	chapNo := bqglll.IDToInt(parts[1])

	if bookId <= 0 || chapNo <= 0 {
		return 0, 0, fmt.Errorf("章节地址缺少有效 ID: %s", rawurl)
	}

	return bookId, chapNo, nil
}

// 简介字段的字符上限
//
// 与 models.Novel 的 orm 定义（size(2555)）保持一致。
// 站点上确有超长简介（实测见 2798 字符），直接入库会报
// "Data too long for column 'desc'"，导致整本书采集失败。
const maxDescLen = 2555

// clipDesc 按字段上限截断简介
//
// 上限由设置项 BqglllMaxDesc 控制，默认等于字段定义（varchar(2555)）。
// 按字符（rune）而非字节截断：字段限制的是字符数，
// 按字节截会把多字节汉字截断成乱码。
func clipDesc(s string) string {
	limit := bqglll.CurrentSettings().MaxDesc
	if limit <= 0 || limit > maxDescLen {
		limit = maxDescLen
	}

	r := []rune(strings.TrimSpace(s))
	if len(r) <= limit {
		return string(r)
	}

	// 截断处尽量落在句末，避免出现半句话
	cut := limit
	for i := limit - 1; i > limit-120 && i > 0; i-- {
		switch r[i] {
		case '。', '！', '？', '…', '；', '.':
			cut = i + 1
		}
		if cut != maxDescLen {
			break
		}
	}

	return string(r[:cut])
}

// mapCateId 按规则的分类映射表把站点分类名转为本地分类 ID
//
// 该站的分类名与本地的并不一致（如站点「玄幻奇幻」对应本地「玄幻魔法」），
// 因此必须在规则里配置 cate_map 做映射。未命中时退回默认分类，
// 避免因分类为空导致整本书保存失败（Save 会校验 CateId 非空）。
func mapCateId(provider *models.SnatchRule, cateName string) uint32 {
	cateName = strings.TrimSpace(cateName)

	// 一级：完全相等
	for _, v := range provider.CateMaps {
		if strings.TrimSpace(v.Name) == cateName {
			return v.Id
		}
	}

	// 二级：前缀匹配
	//
	// 该站的分类存在两套写法：分类页返回全称（玄幻奇幻），
	// 而书籍接口返回简称（玄幻）。规则里通常按全称配置，
	// 因此这里用「映射名以站点分类名开头」来兜住简称的情况。
	// 例如站点给「玄幻」，规则配「玄幻奇幻」→ 匹配成功。
	if cateName != "" {
		for _, v := range provider.CateMaps {
			if strings.HasPrefix(strings.TrimSpace(v.Name), cateName) {
				return v.Id
			}
		}
	}

	// 未命中：用兜底分类，保证仍能入库（Save 会校验 CateId 非空）。
	// 兜底 ID 由设置项 BqglllDefaultCate 控制。
	def := bqglll.CurrentSettings().DefaultCate
	if def == 0 {
		def = DEF_CATE_ID
	}

	log.Debug(fmt.Sprintf("[%s]分类未映射: %q，用兜底 ID=%d",
		provider.Code, cateName, def))

	return def
}

// coverURL 按站点规则拼出封面地址
//
// 规则：bookimg/{floor(id/1000)}/{id}.jpg
func coverURL(id string) string {
	n := bqglll.IDToInt(id)
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("https://www.bqglll.cc/bookimg/%d/%d.jpg", n/1000, n)
}

// textToHTML 把纯文本正文转为与其它采集器一致的 HTML 片段
//
// 站点接口返回的 txt 以 \n 分段。项目内其它采集器的正文均为
// 含 <br/> 的 HTML 片段，前台模板按 HTML 渲染，故此处统一格式。
func textToHTML(txt string) string {
	lines := strings.Split(strings.ReplaceAll(txt, "\r\n", "\n"), "\n")

	var sb strings.Builder
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		// 转义 HTML 特殊字符，避免正文中的尖括号被当作标签
		ln = htmlEscape(ln)
		sb.WriteString("　　")
		sb.WriteString(ln)
		sb.WriteString("<br/>")
	}

	return sb.String()
}

// htmlEscape 转义 HTML 特殊字符
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

/* ---------- 依赖注入（避免包间循环引用） ---------- */

// SetApiPaceProvider 注入采集节奏提供者
//
// 与 CSS 采集器共用同一份节奏配置，保证两种采集器对目标站的行为一致。
func SetApiPaceProvider(f func() (time.Duration, time.Duration)) {
	bqglll.SetPaceProvider(f)
}

// SetApiProxyProvider 注入代理提供者
func SetApiProxyProvider(f func() string) {
	bqglll.SetProxyProvider(f)
}

/* ---------- 采集源设置转发 ---------- */

// ApiSettings 接口型采集器的可调参数（供 services 侧构造）
type ApiSettings = bqglll.Settings

// DefaultApiSettings 默认参数
func DefaultApiSettings() ApiSettings {
	return bqglll.DefaultSettings()
}

// SetApiSettingsProvider 注入设置提供者
func SetApiSettingsProvider(f func() ApiSettings) {
	bqglll.SetSettingsProvider(f)
}

// ApiSettingsNow 取当前设置（供其它模块查询）
func ApiSettingsNow() ApiSettings {
	return bqglll.CurrentSettings()
}
