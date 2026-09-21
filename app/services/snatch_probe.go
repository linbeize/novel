package services

import (
	"time"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/services/snatchs"
	"github.com/vckai/novel/app/utils/log"
)

/*
投毒时段记录与自动试探恢复
==========================

两部分功能：

一、投毒时段记录
    每次因投毒而暂停、以及后续恢复时，各写一条记录到后台操作日志。
    一条「暂停」+ 一条「恢复」即构成一个完整的投毒时段，据此可看出
    站点在什么时间段会投毒（例如每天凌晨固定几小时）。

    用后台操作日志（nov_admin_log）而非应用日志，原因是应用日志只写
    journald，后台界面查不到；操作日志在「后台 - 操作日志」可直接查看。

二、自动试探恢复
    暂停后不再无限期停着，而是定时发起试探请求：

      - 隔 probeInterval 才试一次（避免频繁打扰站点）
      - 连续 probeSuccessNeed 次拿到正常内容，才确认恢复并重新开启采集
      - 任一次试探命中投毒则计数清零、继续等待下一轮

    这样既能在站点恢复后自动继续采集，又不会因偶发一次正常就误判。
*/

const (
	// 日志类型：采集投毒事件（用于后台操作日志的分类展示）
	LogTypePoison = 3301

	// 试探间隔
	probeInterval = 30 * time.Minute

	// 连续多少次试探正常才确认恢复
	probeSuccessNeed = 3

	// 两次试探之间的额外间隔（避免连续打）
	probeGap = 3 * time.Second
)

// 配置键：试探状态
const (
	CfgProbeOK     = "SnatchProbeOK"     // 已连续正常次数
	CfgProbeTotal  = "SnatchProbeTotal"  // 累计试探次数
	CfgProbeLastAt = "SnatchProbeLastAt" // 上次试探时间
)

// PoisonProbe 自动试探服务
type PoisonProbe struct {
	ticker *time.Ticker
	stop   chan struct{}
}

var PoisonProbeService = &PoisonProbe{}

// Start 启动试探循环（幂等）
func (this *PoisonProbe) Start() {
	if this.ticker != nil {
		return
	}

	this.ticker = time.NewTicker(time.Minute)
	this.stop = make(chan struct{})

	go func() {
		for {
			select {
			case <-this.ticker.C:
				this.tryOnce()
			case <-this.stop:
				return
			}
		}
	}()

	log.Info("[投毒试探] 已启动，每", probeInterval, "检查一次")
}

// Stop 停止试探
func (this *PoisonProbe) Stop() {
	if this.ticker != nil {
		this.ticker.Stop()
		this.ticker = nil
	}
	if this.stop != nil {
		close(this.stop)
		this.stop = nil
	}
}

// tryOnce 每次 tick 判断是否该做试探
func (this *PoisonProbe) tryOnce() {
	info := PauseService.Info()
	if !info.Paused {
		// 未处于暂停状态：清空试探计数
		if ConfigService.Int64(CfgProbeOK, 0) != 0 {
			_ = ConfigService.Set(CfgProbeOK, "0")
		}
		return
	}

	// 距上次试探不足 probeInterval 则跳过
	last := ConfigService.Int64(CfgProbeLastAt, 0)
	if last > 0 && time.Since(time.Unix(last, 0)) < probeInterval {
		return
	}

	this.probe(info)
}

// probe 对触发投毒的源发起试探
func (this *PoisonProbe) probe(info PauseInfo) {
	source := info.Source
	if source == "" {
		log.Warn("[投毒试探] 暂停记录缺少来源，跳过试探")
		return
	}

	provider := SnatchRuleService.GetByCode(source)
	if provider == nil {
		log.Warn("[投毒试探] 找不到采集规则:", source)
		return
	}

	// 取一本该源已采集的书做试探，避免凭空构造请求
	novId, chapLink := pickProbeTarget(source)
	if chapLink == "" {
		log.Warn("[投毒试探] 该源暂无已采集书籍，无法试探:", source)
		return
	}

	_ = ConfigService.Set(CfgProbeLastAt, itoa64(time.Now().Unix()))

	total := ConfigService.Int64(CfgProbeTotal, 0) + 1
	_ = ConfigService.Set(CfgProbeTotal, itoa64(total))

	log.Info("[投毒试探] 第", total, "次试探 源:", source, " 小说ID:", novId)

	// 请求章节
	info2, err := SnatchService.GetChapter(source, chapLink)
	if err != nil {
		log.Warn("[投毒试探] 请求失败（视为未恢复）:", err.Error())
		this.resetOK()
		return
	}

	// 检测内容
	pr := snatchs.DetectPoison(info2.Chap.Desc)
	if pr.IsPoison {
		log.Warn("[投毒试探] 第", total, "次仍为投毒内容，继续等待")
		this.resetOK()
		return
	}

	// 本次正常，累计
	ok := ConfigService.Int64(CfgProbeOK, 0) + 1
	_ = ConfigService.Set(CfgProbeOK, itoa64(ok))

	log.Info("[投毒试探] 第", total, "次内容正常（连续", ok, "/", probeSuccessNeed, "）")

	if ok >= probeSuccessNeed {
		this.confirmRecovered(source, total)
	}
}

// resetOK 清零连续正常计数
func (this *PoisonProbe) resetOK() {
	if ConfigService.Int64(CfgProbeOK, 0) != 0 {
		_ = ConfigService.Set(CfgProbeOK, "0")
	}
}

// confirmRecovered 确认恢复并重新开启采集
func (this *PoisonProbe) confirmRecovered(source string, total int64) {
	at := PauseService.PausedAt()

	if err := PauseService.Resume(); err != nil {
		log.Warn("[投毒试探] 恢复采集失败:", err.Error())
		return
	}

	// 记录时段：暂停 → 恢复
	dur := time.Since(at)
	durText := humanDuration(dur)

	writePoisonLog("自动恢复采集",
		"采集源 "+source+" 已连续 "+itoa64(probeSuccessNeed)+" 次返回正常内容，"+
			"自动恢复采集。本次暂停自 "+at.Format("2006-01-02 15:04:05")+
			" 起，共持续 "+durText+"，期间试探 "+itoa64(total)+" 次。")

	log.Info("[投毒试探] 已确认恢复，采集重新开启（暂停持续", durText, "）")

	_ = ConfigService.Set(CfgProbeOK, "0")
	_ = ConfigService.Set(CfgProbeTotal, "0")
	_ = ConfigService.Set(CfgProbeLastAt, "0")
}

/* ---------- 记录投毒事件 ---------- */

// WritePoisonPaused 记录一次「因投毒而暂停」
func WritePoisonPaused(source, reason, sample string) {
	now := time.Now()

	writePoisonLog("采集自动暂停",
		"采集源 "+source+" 返回伪造内容，已自动暂停采集。"+
			"原因："+reason+"。内容样本："+snatchs.CleanSample(sample, 80))

	log.Warn("[投毒记录] 暂停于", now.Format("2006-01-02 15:04:05"), " 源:", source)
}

// writePoisonLog 写后台操作日志
//
// Uid 用 0 会被 Add 拒绝（其要求必须有操作对象），因此这里直接调模型 Insert，
// 并把 Name 记为「系统」，用于区分人工操作与自动事件。
func writePoisonLog(action, content string) {
	m := &models.AdminLog{
		Uid:       0,
		Name:      "系统",
		Ip:        "127.0.0.1",
		Type:      LogTypePoison,
		Content:   "[" + action + "] " + content,
		CreatedAt: uint32(time.Now().Unix()),
		UpdatedAt: uint32(time.Now().Unix()),
	}

	// 长度限制在字段容量内（varchar 255）
	if len([]rune(m.Content)) > 250 {
		m.Content = string([]rune(m.Content)[:250]) + "…"
	}

	if err := m.Insert(); err != nil {
		log.Warn("[投毒记录] 写日志失败:", err.Error())
	}
}

/* ---------- 内部辅助 ---------- */

// pickProbeTarget 取一本该源的书与章节链接用于试探
//
// 取该源下最近添加的一本：它一定已有采集点，章节链接也有效。
// 这样试探用的请求与真实采集完全一致，判定才有意义。
func pickProbeTarget(source string) (uint32, string) {
	m := models.NewNovelLinks()

	list, _ := m.GetAll(models.ArgsNovelLinksList{
		Source: source,
		ArgsBase: models.ArgsBase{
			Limit: 1,
		},
	})

	if len(list) == 0 {
		return 0, ""
	}

	link := list[0]

	// 优先用该书已采集的最后一章：链接确定有效，且内容已知正常，便于比对。
	if chap := ChapterService.GetLast(link.NovId); chap != nil && chap.Link != "" {
		return link.NovId, chap.Link
	}

	// 该书尚无章节（常见于刚添加、尚未采集完的书）。
	// 此时不能直接用采集点地址当章节地址：那是目录页，
	// 传给 GetChapter 会失败。改为先取一次目录，用首章地址试探。
	chaps, err := SnatchService.GetChapters(source, link.Link)
	if err != nil || len(chaps) == 0 {
		return link.NovId, ""
	}

	return link.NovId, chaps[0].Chap.Link
}

// humanDuration 中文时长描述
func humanDuration(d time.Duration) string {
	if d < 0 {
		return "未知"
	}

	sec := int(d.Seconds())
	switch {
	case sec < 60:
		return itoa64(int64(sec)) + " 秒"
	case sec < 3600:
		return itoa64(int64(sec/60)) + " 分钟"
	case sec < 86400:
		h := sec / 3600
		m := (sec % 3600) / 60
		return itoa64(int64(h)) + " 小时 " + itoa64(int64(m)) + " 分钟"
	default:
		day := sec / 86400
		h := (sec % 86400) / 3600
		return itoa64(int64(day)) + " 天 " + itoa64(int64(h)) + " 小时"
	}
}
