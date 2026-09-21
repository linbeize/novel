package services

import (
	"sync"
	"time"

	"github.com/vckai/novel/app/services/snatchs"
	"github.com/vckai/novel/app/utils/log"
)

/*
采集自动暂停
============

投毒检测命中达阈值时，自动关闭采集总开关（IsSnatch），并记录：

  - 暂停时间
  - 触发的采集源
  - 命中原因与内容样本（截断）

这些信息写回配置表，后台首页据此展示提示，便于人工确认站点是否恢复。

为什么用「暂停」而不是「跳过该章继续采」：
  站点进入投毒状态时，后续请求大概率仍是投毒内容。继续采集既浪费时间、
  又可能被站点判定为异常流量，因此选择暂停，等人工确认后再恢复。
*/

const (
	// 配置键：最后暂停时间（Unix 时间戳）
	CfgPauseAt = "SnatchPauseAt"

	// 配置键：暂停原因
	CfgPauseReason = "SnatchPauseReason"

	// 配置键：暂停时的采集源
	CfgPauseSource = "SnatchPauseSource"

	// 配置键：暂停时的内容样本
	CfgPauseSample = "SnatchPauseSample"
)

// pauseService 采集暂停服务
type pauseService struct {
	mu       sync.Mutex
	lastOnce time.Time
}

// PauseService 全局实例
var PauseService = &pauseService{}

// PauseIfPoisoned 若投毒已达阈值则暂停采集
//
// 返回 true 表示本次已触发暂停。
//
// 注意：写入配置时会同步更新内存缓存，因此正在运行的任务
// 在下一轮 gc/调度时即会看到 IsSnatch=0 而停止。
func (this *pauseService) PauseIfPoisoned(source, reason, sample string) bool {
	// 阈值判断由 snatchs 包的跟踪器负责：连续命中达阈值才暂停，
	// 避免偶发一次异常就中断整个采集。
	if snatchs.GetPoisonStatus(source).Streak < snatchs.PoisonThreshold {
		return false
	}

	this.mu.Lock()
	defer this.mu.Unlock()

	// 避免同一波投毒重复写入（60 秒内只记一次）
	if time.Since(this.lastOnce) < time.Minute {
		return true
	}
	this.lastOnce = time.Now()

	now := time.Now()

	// 记录暂停时间与上下文
	_ = ConfigService.Set(CfgPauseAt, itoa64(now.Unix()))
	_ = ConfigService.Set(CfgPauseReason, reason)
	_ = ConfigService.Set(CfgPauseSource, source)
	_ = ConfigService.Set(CfgPauseSample, clipSample(sample, 200))

	// 关闭采集总开关
	_ = ConfigService.Set("IsSnatch", "0")

	log.Warn("[自动暂停] 检测到采集站返回伪造内容，已暂停采集")
	log.Warn("[自动暂停] 时间:", now.Format("2006-01-02 15:04:05"),
		" 来源:", source, " 原因:", reason)

	// 写入后台操作日志，便于事后查看投毒发生在哪些时间段
	WritePoisonPaused(source, reason, sample)

	return true
}

// IsPaused 当前是否处于「因投毒而暂停」状态
func (this *pauseService) IsPaused() bool {
	return ConfigService.String(CfgPauseAt) != "" &&
		!ConfigService.Bool("IsSnatch", true)
}

// PausedAt 取暂停时间
func (this *pauseService) PausedAt() time.Time {
	v := ConfigService.Int64(CfgPauseAt, 0)
	if v <= 0 {
		return time.Time{}
	}
	return time.Unix(v, 0)
}

// PauseInfo 暂停信息（供后台展示）
type PauseInfo struct {
	Paused  bool
	At      time.Time
	AtText  string // 格式化的时间，模板可直接用
	AgoText string // 距今多久，便于判断是否为新发生的
	Source  string
	Reason  string
	Sample  string
}

// Info 取暂停信息
func (this *pauseService) Info() PauseInfo {
	at := this.PausedAt()

	info := PauseInfo{
		Paused: this.IsPaused() && !at.IsZero(),
		At:     at,
		Source: ConfigService.String(CfgPauseSource),
		Reason: ConfigService.String(CfgPauseReason),
		Sample: ConfigService.String(CfgPauseSample),
	}

	if !at.IsZero() {
		info.AtText = at.Format("2006-01-02 15:04:05")
		info.AgoText = humanAgo(time.Since(at))
	}

	return info
}

// Resume 人工恢复采集（清除暂停标记）
func (this *pauseService) Resume() error {
	this.mu.Lock()
	this.lastOnce = time.Time{}
	this.mu.Unlock()

	_ = ConfigService.Set(CfgPauseAt, "")
	_ = ConfigService.Set(CfgPauseReason, "")
	_ = ConfigService.Set(CfgPauseSource, "")
	_ = ConfigService.Set(CfgPauseSample, "")

	if err := ConfigService.Set("IsSnatch", "1"); err != nil {
		return err
	}

	log.Info("[自动暂停] 已手动恢复采集")
	return nil
}

/* ---------- 内部辅助 ---------- */

// humanAgo 把时长转成中文描述
func humanAgo(d time.Duration) string {
	if d < 0 {
		return "刚刚"
	}

	sec := int(d.Seconds())
	switch {
	case sec < 60:
		return "刚刚"
	case sec < 3600:
		return itoa64(int64(sec/60)) + " 分钟前"
	case sec < 86400:
		return itoa64(int64(sec/3600)) + " 小时前"
	default:
		return itoa64(int64(sec/86400)) + " 天前"
	}
}

// clipSample 截断样本并清理换行，避免污染配置值
func clipSample(s string, n int) string {
	s = snatchs.CleanSample(s, n)
	return s
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
