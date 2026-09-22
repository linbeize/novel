package log

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego"
)

/*
后台日志查看支持
================

应用的日志原本只写到标准输出（部署后由 systemd journal 收集），
在后台页面上看不到。这里在写日志的同时，把最近若干条同时存入一个
内存环形缓冲区，供后台「运行日志」页面读取。

设计取舍：
  - 只保留内存，不落盘：重启即清空，也不会长期占用磁盘；
    需要留存的历史日志仍可由 journal 提供（journalctl -u novel）。
  - 固定容量环形缓冲：容量满后覆盖最旧的记录，内存占用恒定。
  - 并发安全：日志可能来自多个采集协程，读写都用互斥锁保护。
*/

// 缓冲区容量（条）
const ringCapacity = 2000

// Entry 一条日志记录
type Entry struct {
	Seq     uint64 `json:"seq"`     // 自增序号，前端据此判断是否有新日志
	Time    string `json:"time"`    // 格式化时间
	Level   string `json:"level"`   // 级别（DEBUG/INFO/WARN/ERROR）
	Message string `json:"message"` // 正文
}

var (
	ringMu   sync.RWMutex
	ring     = make([]Entry, 0, ringCapacity)
	ringSeq  uint64
	ringHead int // 下一个写入位置
	ringFull bool
)

// 写入环形缓冲区
func appendRing(level, msg string) {
	ringMu.Lock()
	defer ringMu.Unlock()

	ringSeq++

	e := Entry{
		Seq:     ringSeq,
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Level:   level,
		Message: msg,
	}

	if len(ring) < ringCapacity {
		ring = append(ring, e)
		return
	}

	// 已满：覆盖最旧的一条
	ring[ringHead] = e
	ringHead = (ringHead + 1) % ringCapacity
	ringFull = true
}

// GetEntries 取日志记录（按时间正序，即最旧在前）
//
// level: 过滤级别，为空或 "ALL" 时返回全部；
//
//	否则只返回该级别及更严重的日志（如传 WARN 会返回 WARN/ERROR）。
//
// limit: 最多返回条数，<=0 时使用默认值 500。
// afterSeq: 只返回序号大于该值的记录，用于前端增量拉取；为 0 时返回全部。
func GetEntries(level string, limit int, afterSeq uint64) []Entry {
	ringMu.RLock()
	defer ringMu.RUnlock()

	if limit <= 0 {
		limit = 500
	}

	// 按顺序取出（环形已满时从 ringHead 开始）
	ordered := make([]Entry, 0, len(ring))
	if ringFull && len(ring) == ringCapacity {
		ordered = append(ordered, ring[ringHead:]...)
		ordered = append(ordered, ring[:ringHead]...)
	} else {
		ordered = append(ordered, ring...)
	}

	// 级别过滤
	minLevel := levelRank(level)
	filtered := make([]Entry, 0, len(ordered))
	for _, e := range ordered {
		if afterSeq > 0 && e.Seq <= afterSeq {
			continue
		}
		if minLevel > 0 && levelRank(e.Level) < minLevel {
			continue
		}
		filtered = append(filtered, e)
	}

	// 只保留最新的 limit 条
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered
}

// Seq 当前最新序号，供前端判断是否有新日志
func Seq() uint64 {
	ringMu.RLock()
	defer ringMu.RUnlock()
	return ringSeq
}

// Clear 清空缓冲区
func Clear() {
	ringMu.Lock()
	defer ringMu.Unlock()

	ring = make([]Entry, 0, ringCapacity)
	ringHead = 0
	ringFull = false
	// 注意：不重置 ringSeq，避免前端把旧日志误判为新日志
}

// 级别权重，数值越大越严重；未知级别返回 0
func levelRank(level string) int {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return 1
	case "INFO":
		return 2
	case "NOTICE":
		return 3
	case "WARN", "WARNING":
		return 4
	case "ERROR":
		return 5
	default:
		return 0
	}
}

// 把可变参数拼接为单行文本
//
// 单个字符串参数原样返回；多个参数则依次拼接。
//
// 这里没有直接用 fmt.Sprintln：它在每个参数之间都插入空格，而本项目的
// 调用普遍形如 log.Error("获取失败：", err)，用 Sprintln 会得到
// 「获取失败： xxx」——中文冒号后多一个多余空格。
// 故规则为：前一个参数以空白结尾、或后一个参数以空白开头时不再补空格，
// 其余情况补一个空格，兼顾中文标点与英文单词的拼接。
func format(v ...interface{}) string {
	if len(v) == 0 {
		return ""
	}
	if len(v) == 1 {
		return fmt.Sprint(v[0])
	}

	var sb strings.Builder
	for i, item := range v {
		part := fmt.Sprint(item)
		if i > 0 && part != "" {
			prev := sb.String()
			if prev != "" &&
				!strings.HasSuffix(prev, " ") && !strings.HasSuffix(prev, "\n") &&
				!strings.HasPrefix(part, " ") {
				// 前一个字符是中文标点时不补空格
				if !isChinesePunct(prev) {
					sb.WriteByte(' ')
				}
			}
		}
		sb.WriteString(part)
	}

	return sb.String()
}

// 判断字符串结尾是否为中文标点（这类标点后不需要再补空格）
func isChinesePunct(s string) bool {
	if s == "" {
		return false
	}
	r := []rune(s)
	switch r[len(r)-1] {
	case '：', '，', '。', '、', '；', '！', '？', '（', '【', '“', '《':
		return true
	}
	return false
}

// 错误日记
func Error(v ...interface{}) {
	appendRing("ERROR", format(v...))
	beego.Error(v...)
}

// 警告日记
func Warn(v ...interface{}) {
	appendRing("WARN", format(v...))
	beego.Warning(v...)
}

// 注意日记
func Notice(v ...interface{}) {
	appendRing("NOTICE", format(v...))
	beego.Notice(v...)
}

// 正常信息日记
func Info(v ...interface{}) {
	appendRing("INFO", format(v...))
	beego.Info(v...)
}

// 调试日记
//
// 该级别日志量大（采集时逐条记录，其中多数是页面上的 javascript:/站外
// 链接这类预期情况），生产环境默认关闭。
//
// 关闭时既不写控制台、也不进内存缓冲——否则后台「日志」页仍会被这些
// 内容刷屏，等于没关。需要查看时把后台「系统设置 - 日志级别」改为
// debug 即可（该项为空即视为非 debug）。
func Debug(v ...interface{}) {
	if !debugEnabled() {
		return
	}
	appendRing("DEBUG", format(v...))
	beego.Debug(v...)
}

// debugEnabled 是否记录 DEBUG 级别日志
//
// 由外部注入的判定函数决定（见 SetDebugProvider）。这样做的原因：
// LogLevel 存放在数据库里，而读取它需要 services 包，本包被 services 依赖，
// 直接引用会形成循环。因此改为注入，由启动时在 services 侧完成绑定。
//
// 采用每次调用求值，因此后台修改日志级别后无需重启即可生效。
func debugEnabled() bool {
	if debugProvider == nil {
		return false
	}
	return debugProvider()
}

// debugProvider 由 SetDebugProvider 注入
var debugProvider func() bool

// SetDebugProvider 注入 DEBUG 级别的判定函数
func SetDebugProvider(f func() bool) {
	debugProvider = f
}
