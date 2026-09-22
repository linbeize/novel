package snatchs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/axgle/mahonia"

	xhttp "github.com/vckai/novel/app/librarys/net/http"
	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/utils"
	"github.com/vckai/novel/app/utils/log"
)

/*
bqgnovels 搜索适配
==================

背景
----
该站原本是服务端渲染，搜索地址为 /search?keyword=xxx，采集规则用 CSS
选择器抓结果。2026-09 前后该站迁移到 Nuxt（前端渲染 + JSON 接口），
旧的搜索地址返回 404，导致：

  - 后台「搜索采集站」里该站始终无结果
  - 无法通过搜索添加新书

但**采集链路仍然可用**：书籍页 /book/{id}/{n} 仍能正常取到目录与正文
（实测 1010 章、正文正常）。因此仅需修复搜索这一环。

可用的接口
----------
实测该站的 JSON 接口（其余参数名一致）：

	GET /api/query/search?keyword=xxx     搜索书籍
	GET /api/query/get_book_list?bookId=  书籍详情（含目录）
	GET /api/query/get_list               书籍列表

搜索返回结构：

	{"code":200,"data":{"page":1,"size":20,"count":2,"list":[
	  {"id":"34819","title":"斗破苍穹","author":"天蚕土豆","des":"...",
	   "update_content":"土豆大结局感言","state":"1","class_id":"1",
	   "click":"144590","imgUrl":"https://.../34819.jpg"}
	]}}

其中 id 即书籍页地址所用的编号（/book/{id}/1），imgUrl 为封面。

适配方式
--------
不引入新的采集器类型，而是在 CSS 采集器的 FindNovel 之前加一层判断：
当规则的 find_url 指向已失效的搜索地址时，改用接口获取结果。
这样规则的其它字段（采集点格式、目录选择器等）全部照旧复用。
*/

// bqgnovelsSearchAPI 该站的搜索接口
const bqgnovelsSearchAPI = "https://www.bqgnovels.com/api/query/search"

// bqgnovelsBookURL 该站的书籍页地址模板
const bqgnovelsBookURL = "https://www.bqgnovels.com/book/%s"

// isDeadBqgnovelsSearch 判断规则是否指向已失效的搜索地址
//
// 该站改版后 /search?... 返回 404，此时改走接口。
func isDeadBqgnovelsSearch(provider *models.SnatchRule) bool {
	if provider == nil || provider.Rules == nil {
		return false
	}

	fu := strings.TrimSpace(provider.Rules.FindURL)

	return strings.Contains(fu, "bqgnovels.com/search")
}

// searchBqgnovelsViaAPI 通过接口搜索该站
func (this *Snatch) searchBqgnovelsViaAPI(provider *models.SnatchRule, kw string, limit int) ([]*SnatchInfo, error) {
	if limit <= 0 {
		limit = 20
	}

	u := bqgnovelsSearchAPI + "?keyword=" + url.QueryEscape(kw)

	// 注意：不能用 newHtml——它把响应解析成 HTML 文档，JSON 会被
	// 当成无标签文本处理而丢失结构。这里取原始响应体。
	body, err := this.fetchRaw(u, provider.Charset)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("搜索接口无响应")
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Count int `json:"count"`
			List  []struct {
				// id 在响应里是数字（如 34819），但其它字段是字符串。
				// 该站不同接口对 id 的类型并不一致，用 json.Number
				// 接收可同时兼容数字与字符串两种写法。
				ID            json.Number `json:"id"`
				Title         string      `json:"title"`
				Author        string      `json:"author"`
				Des           string      `json:"des"`
				UpdateContent string      `json:"update_content"`
				ImgURL        string      `json:"imgUrl"`
			} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf("搜索失败: %s", resp.Msg)
	}

	out := make([]*SnatchInfo, 0, len(resp.Data.List))

	for i, it := range resp.Data.List {
		if i >= limit {
			break
		}
		id := it.ID.String()
		if id == "" || id == "0" || it.Title == "" {
			continue
		}

		nov := models.NewNovel()
		nov.Name = it.Title
		nov.Author = it.Author
		nov.Desc = this.filter(provider.Rules.BookDescFilter, it.Des)
		nov.Desc = clipDesc(nov.Desc)
		nov.ChapterTitle = it.UpdateContent
		nov.Cover = it.ImgURL

		link := fmt.Sprintf(bqgnovelsBookURL, id)

		out = append(out, &SnatchInfo{
			Nov:        nov,
			Url:        link,
			Source:     provider.Code,
			SiteName:   provider.Name,
			ChapterUrl: link,
		})
	}

	log.Debug(fmt.Sprintf("[%s]接口搜索[%s]得到 %d 条", provider.Code, kw, len(out)))

	return out, nil
}

// proxyURL 取当前代理地址
func (this *Snatch) proxyURL() string {
	if this.proxyFunc == nil {
		return ""
	}
	return this.proxyFunc()
}

// fetchRaw 抓取原始响应体（不解析为 HTML）
//
// 用于 JSON 接口：newHtml 会把内容交给 goquery 解析，
// JSON 结构在 HTML 解析过程中会丢失，故单独提供本方法。
func (this *Snatch) fetchRaw(rawurl, charset string) (string, error) {
	conf := &xhttp.ClientConfig{
		Timeout:   30 * time.Second,
		Dial:      15 * time.Second,
		KeepAlive: 60 * time.Second,
		ProxyURL:  this.proxyURL(),
	}

	c := xhttp.NewClient(conf)

	// 复用注入的采集节奏（与其它采集请求一致，避免超出站点承受）
	interval, jitter := snatchPace()

	throttle := utils.ThrottleInstance()
	throttle.Wait(rawurl, interval, jitter)
	res, _, err := c.Get(context.TODO(), rawurl, nil)
	throttle.Done(rawurl)

	if err != nil {
		return "", err
	}

	// 编码转换（该站为 UTF-8，保留转换逻辑以兼容 GBK 站点）
	if charset != "" && charset != "UTF-8" {
		enc := mahonia.NewDecoder("GB18030")
		if enc != nil {
			return enc.ConvertString(string(res)), nil
		}
	}

	return string(res), nil
}
