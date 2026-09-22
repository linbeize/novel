package snatchs

import (
	"fmt"
	"strings"

	"github.com/vckai/novel/app/services/snatchs/bqglll"
	"github.com/vckai/novel/app/utils/log"
)

/*
分类页批量取书（bqglll）
========================

背景
----
该站的分类页（/xuanhuan/ 等）服务端只渲染 15 本，更多内容靠前端 JS
调 /json?sortid=N&page=M 加载。因此仅靠「抓 HTML 里的 <a> 链接」这种
通用爬虫方式，每个分类只能拿到 15 本，7 个分类合计约 105 本。

而 /json 接口可用相同参数翻到约 20-30 页（每页 15 本），
按 7 个分类估算可覆盖约 2000+ 本，且返回的数据里带封面图路径
（含内部 ID），不必再去解析 HTML。

为什么不用排行榜
----------------
站点的 /top/ 排行榜虽有多达 370 本，但实测其中混有大量非小说内容
（成人向短篇等），采集后书名与内容均不符合预期。故这里只走 7 个
正式分类页，不碰排行榜。

接口说明
--------
	GET /json?sortid={分类号}&page={页码}

	sortid 取自分类页底部 loadmore('N') 的实参，实测为：
	  1 玄幻  2 武侠  3 都市  4 历史  5 网游  6 科幻  7 女生

	返回 JSON 数组，每项含 url_list（SEO 页面地址）、url_img（封面，
	含内部 ID）、articlename（书名）、author、intro。

	页码超出范围时返回空数组，可作为翻页终止条件。
*/

// bqglllSortIDs 各分类的 sortid（取自分类页的 loadmore 调用）
var bqglllSortIDs = []struct {
	ID   int
	Name string
}{
	{1, "玄幻奇幻"},
	{2, "武侠仙侠"},
	{3, "都市言情"},
	{4, "历史军事"},
	{5, "网游竞技"},
	{6, "科幻灵异"},
	{7, "女生频道"},
}

// 每页条数（站点固定 15 条，此处仅用于估算进度）
const bqglllPageSize = 15

// 单次采集最多翻多少页（防止站点页码异常导致死循环）
const bqglllMaxPage = 60

// CrawlCategories 遍历全部分类页，把发现的书籍地址送入回调
//
// found 回调收到的是 SEO 页面地址（如 https://www.bqglll.cc/look/183437/），
// 由上层交给通用流程处理（换算内部 ID → 建采集点）。
//
// 返回累计发现的书籍数。
func (this *ApiSnatch) CrawlCategories(found func(link string) bool) int {
	total := 0

	for _, c := range bqglllSortIDs {
		n := this.crawlOneCategory(c.ID, c.Name, found)
		total += n

		log.Info(fmt.Sprintf("[%s]分类页采集完成: %s 共 %d 本",
			"bqglll", c.Name, n))
	}

	return total
}

// crawlOneCategory 翻页采集单个分类
func (this *ApiSnatch) crawlOneCategory(sortID int, name string, found func(string) bool) int {
	count := 0

	for page := 1; page <= bqglllMaxPage; page++ {
		items, err := this.client.GetSortPage(sortID, page)
		if err != nil {
			log.Warn(fmt.Sprintf("[bqglll]分类页请求失败: %s page=%d %v",
				name, page, err))
			break
		}

		// 空数组表示已到末页
		if len(items) == 0 {
			break
		}

		for _, it := range items {
			link := this.normalizeSortURL(it.URLList)
			if link == "" {
				continue
			}

			count++
			if !found(link) {
				// 回调返回 false 表示已存在，跳过但继续翻页
				continue
			}
		}
	}

	return count
}

// normalizeSortURL 把 /json 返回的相对地址补成完整地址
func (this *ApiSnatch) normalizeSortURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}

	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}

	if strings.HasPrefix(u, "/") {
		return providerHost + u
	}

	return providerHost + "/" + u
}

// SortItems 分类页条目（导出别名，便于上层使用）
type SortItems = bqglll.SortItem
