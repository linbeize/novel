package snatchs

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

/*
采集内容投毒检测
================

背景
----
部分采集站会对「疑似爬虫」的请求返回**伪造正文**，而非其真实内容。
实测案例（bqglll.cc）：正常时段返回真正文；投毒时段则对同一章节返回
随机生成的色情软文，且每次请求内容都不同。其特点是：

  - 所有被投毒的章节都以同一句话开头：
    「我今年２２岁，是一个大学阿拉伯语专业的学生…」
  - 正文中随机插入导流域名水印（形如 xxx♜cc、yyy♀com）
  - 标题仍然正确（按目录返回），只有正文被替换 ⇒ 只查标题发现不了

这类内容一旦入库会污染书库，且因每次内容随机，事后再想清理很难
（无法靠哈希去重）。因此需要在**入库前**拦截。

处理策略
--------
  1. 检测命中 → 该章不入库，计入失败并重试
  2. 连续多次命中 → 判定为「站点进入投毒状态」，暂停采集
     （避免继续请求既浪费时间、又可能被站点视为异常流量）
  3. 暂停时记录具体时间与命中样本，供后台展示与人工核查

暂停采用项目既有的总开关 IsSnatch，因此前台/后台的既有逻辑无需改动。
*/

// 投毒内容的特征开头（去除空白后匹配）
//
// 该句在实测的所有投毒样本中完全一致，是最可靠的判别依据。
var poisonHeaders = []string{
	"我今年２２岁，是一个大学阿拉伯语专业的学生",
	"我今年22岁，是一个大学阿拉伯语专业的学生",
}

// 导流域名水印
//
// 形如：bqg82○ de / caxao♀com / ars8♜cc / srsp·cc
// 特征为「字母数字片段 + 非标准分隔符 + 2-3 位顶级域」。
// 正常小说正文中不会出现这种形态。
var watermarkRe = regexp.MustCompile(
	`[a-zA-Z0-9]{3,}\s?[ヽ·.●♜♀⊙◇♟○•,，]\s?[a-zA-Z]{2,4}`,
)

// 成人内容词
//
// 单独出现不足以判定（正常小说也可能提及），
// 但配合水印或特征开头即可确认。因此仅在「水印 + 成人词」同时命中时判定。
var adultWords = []string{
	"阴茎", "阴道", "龟头", "乳房", "口交", "射精", "淫水", "抽插",
}

// 检测结果
type PoisonResult struct {
	IsPoison bool
	Reason   string // 命中原因，便于日志与排查
}

// DetectPoison 检测章节正文是否为投毒内容
func DetectPoison(content string) PoisonResult {
	if content == "" {
		return PoisonResult{}
	}

	// 去掉 HTML 标签与空白后再判断，避免标签干扰
	plain := stripTags(content)
	compact := strings.Join(strings.Fields(plain), "")

	// 1. 特征开头（最可靠）
	for _, h := range poisonHeaders {
		if strings.Contains(compact, strings.ReplaceAll(h, " ", "")) {
			return PoisonResult{true, "命中投毒特征句"}
		}
	}

	// 2. 水印 + 成人词组合
	marks := watermarkRe.FindAllString(plain, -1)
	if len(marks) > 0 {
		for _, w := range adultWords {
			if strings.Contains(plain, w) {
				return PoisonResult{
					true,
					"命中导流水印(" + strings.Join(uniqN(marks, 3), ",") + ")+成人词",
				}
			}
		}
	}

	return PoisonResult{}
}

// stripTags 去掉 HTML 标签并反转义常见实体
func stripTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}

	out := b.String()
	r := strings.NewReplacer(
		"&nbsp;", " ",
		"&lt;", "<",
		"&gt;", ">",
		"&amp;", "&",
		"&quot;", `"`,
		"\u00a0", " ",
	)
	return r.Replace(out)
}

// uniqN 取前 n 个去重元素
func uniqN(in []string, n int) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) >= n {
			break
		}
	}
	return out
}

/*
投毒状态跟踪
============

按采集源分别记录。当某源连续命中投毒达到阈值时，判定该源进入投毒状态，
由调用方决定是否暂停全局采集。
*/

// 连续命中多少次后判定为投毒状态
const poisonThreshold = 3

type poisonTracker struct {
	mu sync.Mutex

	// 各源的连续命中次数
	streak map[string]int

	// 各源最近一次命中时间与样本
	lastAt     map[string]time.Time
	lastReason map[string]string
	lastSample map[string]string
}

var tracker = &poisonTracker{
	streak:     map[string]int{},
	lastAt:     map[string]time.Time{},
	lastReason: map[string]string{},
	lastSample: map[string]string{},
}

// PoisonObserved 记录一次投毒命中
//
// 返回 true 表示已达阈值，调用方应暂停采集。
func PoisonObserved(source string, res PoisonResult, sample string) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	tracker.streak[source]++
	tracker.lastAt[source] = time.Now()
	tracker.lastReason[source] = res.Reason
	tracker.lastSample[source] = clip(sample, 120)

	return tracker.streak[source] >= poisonThreshold
}

// PoisonCleared 记录一次正常内容，重置该源的连续计数
func PoisonCleared(source string) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	delete(tracker.streak, source)
	delete(tracker.lastAt, source)
	delete(tracker.lastReason, source)
	delete(tracker.lastSample, source)
}

// PoisonStatus 取某源的投毒状态
type PoisonStatus struct {
	Streak int       // 连续命中次数
	LastAt time.Time // 最近命中时间
	Reason string    // 命中原因
	Sample string    // 命中样本摘要
}

// GetPoisonStatus 查询指定源的投毒状态
func GetPoisonStatus(source string) PoisonStatus {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	return PoisonStatus{
		Streak: tracker.streak[source],
		LastAt: tracker.lastAt[source],
		Reason: tracker.lastReason[source],
		Sample: tracker.lastSample[source],
	}
}

// AllPoisonStatus 查询所有源的投毒状态
func AllPoisonStatus() map[string]PoisonStatus {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	out := map[string]PoisonStatus{}
	for k := range tracker.streak {
		out[k] = PoisonStatus{
			Streak: tracker.streak[k],
			LastAt: tracker.lastAt[k],
			Reason: tracker.lastReason[k],
			Sample: tracker.lastSample[k],
		}
	}
	return out
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(stripTags(s)), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// PoisonThreshold 连续命中多少次判定为投毒状态
//
// 默认值见 poisonThreshold；实际阈值可由设置项调整（见 snatchs 包的
// SetPoisonThresholdProvider）。这里保留函数形态以保证调用方不变。
func PoisonThreshold() int {
	if thresholdProvider != nil {
		if n := thresholdProvider(); n > 0 {
			return n
		}
	}
	return poisonThreshold
}

// thresholdProvider 由 SetPoisonThresholdProvider 注入
var thresholdProvider func() int

// SetPoisonThresholdProvider 注入阈值提供者
func SetPoisonThresholdProvider(f func() int) {
	thresholdProvider = f
}

// CleanSample 清理并截断样本，便于写入配置或日志
func CleanSample(s string, n int) string {
	return clip(s, n)
}
