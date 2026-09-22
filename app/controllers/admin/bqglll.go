package admin

import (
	"strconv"
	"strings"

	"github.com/vckai/novel/app/services"
)

/*
bqglll 采集源设置页
==================

把该源特有的可调参数集中到一处，避免混在全局配置里难以查找。

包含三类：
  1. 采集节奏（间隔/抖动）—— 该站响应较慢，间隔需要单独放宽
  2. 内容处理（简介截断、分类兜底）—— 站点数据不规范时的兜底策略
  3. 投毒防护（检测开关、试探间隔）—— 该站特有的风险

配置键统一以 Bqglll 作前缀存于 nov_config，与全局配置互不影响。
*/

// BqglllController 采集源设置
type BqglllController struct {
	BaseController
}

// 该源的配置项定义（键、默认值、说明）
//
// 前端据此渲染表单，后端据此校验与落库，避免两处各写一份。
var bqglllConfigDefs = []struct {
	Key     string
	Default string
	Label   string
	Tip     string
}{
	// —— 采集节奏 ——
	{"BqglllInterval", "2500", "采集间隔（毫秒）",
		"两次请求之间的最小间隔。该站响应较慢（实测单章 3-6 秒），间隔过小易触发限流。建议 2000-4000。"},
	{"BqglllJitter", "1200", "间隔抖动（毫秒）",
		"在间隔基础上叠加的随机值，使请求节奏不固定，降低被识别为爬虫的概率。"},
	{"BqglllTimeout", "30", "单次请求超时（秒）",
		"该站偶发响应缓慢，超时太短会导致大量重试失败。建议 20-40。"},
	{"BqglllRetry", "3", "请求失败重试次数",
		"网络抖动或站点偶发 502 时的重试次数。实测该站会出现瞬时的 502。"},

	// —— 内容处理 ——
	{"BqglllMaxDesc", "2555", "简介最大长度（字符）",
		"超过则截断。数据库 desc 字段为 varchar(2555)，站点上有更长的简介（实测 2798 字符），不截断会导致整本书采集失败。"},
	{"BqglllDefaultCate", "13", "未匹配分类时的兜底 ID",
		"站点分类名与本地不一致且未在规则的分类映射里配置时，使用该分类，避免因分类为空而无法入库。"},

	// —— 投毒防护 ——
	{"BqglllPoisonCheck", "1", "启用投毒内容检测",
		"该站会间歇性返回伪造正文（标题正确、内容为随机软文）。开启后检测到即不入库，达阈值自动暂停采集。1=开启 0=关闭"},
	{"BqglllPoisonThreshold", "3", "连续命中多少次暂停",
		"连续检测到投毒内容达到该次数时暂停采集，避免偶发一次异常就中断。建议 3-5。"},
	{"BqglllProbeInterval", "30", "恢复试探间隔（分钟）",
		"暂停后每隔多久试探一次站点是否恢复。连续 3 次正常内容才自动恢复采集。"},
}

// Index 设置页面
func (this *BqglllController) Index() {
	// 该源的运行状态
	this.Data["State"] = this.sourceState()

	// 当前配置值（含默认值）
	type item struct {
		Key   string
		Value string
		Label string
		Tip   string
	}
	var items []item
	for _, d := range bqglllConfigDefs {
		items = append(items, item{
			Key:   d.Key,
			Value: services.ConfigService.String(d.Key, d.Default),
			Label: d.Label,
			Tip:   d.Tip,
		})
	}
	this.Data["Items"] = items

	// 投毒状态
	this.Data["Poison"] = services.PauseService.Info()

	this.View("bqglll/index.tpl")
}

// Save 保存设置
func (this *BqglllController) Save() {
	saved := 0

	for _, d := range bqglllConfigDefs {
		v := strings.TrimSpace(this.GetString(d.Key))
		if v == "" {
			v = d.Default
		}

		// 数值项做范围校验，避免填入非法值导致采集异常
		if n, err := strconv.Atoi(v); err == nil {
			v = this.clamp(d.Key, n)
		} else if this.isNumeric(d.Key) {
			// 数值项填了非数字，回落默认值
			v = d.Default
		}

		if err := services.ConfigService.SetOrCreate(d.Key, v); err != nil {
			this.OutJson(1001, "保存失败："+d.Label+" "+err.Error())
			return
		}
		saved++
	}

	this.AddLog(3302)
	this.OutJson(0, "已保存 "+strconv.Itoa(saved)+" 项设置")
}

// Reset 恢复默认值
func (this *BqglllController) Reset() {
	for _, d := range bqglllConfigDefs {
		_ = services.ConfigService.SetOrCreate(d.Key, d.Default)
	}

	this.AddLog(3302)
	this.OutJson(0, "已恢复默认设置")
}

/* ---------- 内部辅助 ---------- */

// isNumeric 该配置项是否为数值型
func (this *BqglllController) isNumeric(key string) bool {
	return key != "BqglllPoisonCheck" // 其余均为数值
}

// clamp 对数值项做合理范围约束
func (this *BqglllController) clamp(key string, n int) string {
	lo, hi := 0, 0

	switch key {
	case "BqglllInterval":
		lo, hi = 200, 60000
	case "BqglllJitter":
		lo, hi = 0, 30000
	case "BqglllTimeout":
		lo, hi = 5, 300
	case "BqglllRetry":
		lo, hi = 0, 10
	case "BqglllMaxDesc":
		lo, hi = 100, 2555 // 不得超过字段上限
	case "BqglllDefaultCate":
		lo, hi = 1, 10000
	case "BqglllPoisonCheck":
		lo, hi = 0, 1
	case "BqglllPoisonThreshold":
		lo, hi = 1, 20
	case "BqglllProbeInterval":
		lo, hi = 5, 1440
	default:
		return strconv.Itoa(n)
	}

	if n < lo {
		n = lo
	}
	if n > hi {
		n = hi
	}

	return strconv.Itoa(n)
}

// sourceState 该源的运行状态摘要
func (this *BqglllController) sourceState() map[string]interface{} {
	out := map[string]interface{}{
		"Exists": false,
		"State":  0,
	}

	rule := services.SnatchRuleService.GetByCode("bqglll")
	if rule == nil {
		return out
	}

	out["Exists"] = true
	out["Name"] = rule.Name
	out["State"] = rule.State
	out["Url"] = rule.Url

	return out
}
