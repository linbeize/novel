// Copyright 2017 Vckai Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/astaxie/beego/orm"
)

// 请求级性能统计
//
// 在页面底部展示本次请求的服务端耗时（以及可选的 SQL 次数），
// 便于直观判断瓶颈在数据库、模板渲染还是其它环节。
//
// 设计取舍：
//
//  1. 耗时统计始终开启且开销极低（仅记录起止时间）。
//
//  2. SQL 次数统计默认关闭。原因：beego ORM 只有在 orm.Debug=true 时
//     才会记录查询日志，而该模式下每条 SQL 都会做一次完整的字符串
//     格式化（拼出完整 SQL 与全部参数），这在生产环境会带来可观开销，
//     反而拖慢页面。因此将其做成可选开关（后台 PerfSQLCount），
//     仅在你需要排查「某页到底查了多少次库」时临时开启。
//
//  3. SQL 归属按 goroutine 区分：ORM 日志由哪个 goroutine 写出，
//     就记到哪个请求上，避免并发时互相串数。
type Perf struct {
	start time.Time

	sqlNum  int64 // 原子累加
	sqlTime int64 // 纳秒，原子累加
}

// 按 goroutine id 索引当前请求的统计对象
var (
	perfMapMu sync.RWMutex
	perfMap   = make(map[uint64]*Perf)

	// SQL 统计开关（由配置注入）
	sqlCountEnabled int32
)

// 启用/关闭 SQL 次数统计
//
// 注意：开启需要同时打开 orm.Debug，会带来每条 SQL 的格式化开销，
// 仅建议在排查问题时临时使用。
func EnableSQLCount(on bool) {
	if on {
		atomic.StoreInt32(&sqlCountEnabled, 1)
	} else {
		atomic.StoreInt32(&sqlCountEnabled, 0)
	}
}

func sqlCountOn() bool {
	return atomic.LoadInt32(&sqlCountEnabled) == 1
}

// 取得当前 goroutine 的 id
//
// Go 未公开 goroutine id，这里从 runtime.Stack 的头部解析
// （格式固定为 "goroutine 123 [running]:"）。
// 仅在请求开始/结束以及每条 SQL 日志时调用，开销可接受。
func goid() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)

	// 形如: "goroutine 123 [running]:"
	s := string(buf[:n])
	s = strings.TrimPrefix(s, "goroutine ")
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		return 0
	}

	id, err := strconv.ParseUint(s[:i], 10, 64)
	if err != nil {
		return 0
	}

	return id
}

// 开始统计（控制器 Prepare 阶段调用）
func StartPerf() *Perf {
	p := &Perf{start: time.Now()}

	perfMapMu.Lock()
	perfMap[goid()] = p
	perfMapMu.Unlock()

	return p
}

// 结束统计（控制器 Finish 阶段调用，及时释放避免 map 泄漏）
func (p *Perf) Stop() {
	if p == nil {
		return
	}

	perfMapMu.Lock()
	delete(perfMap, goid())
	perfMapMu.Unlock()
}

// 当前 goroutine 对应的统计对象
func currentPerf() *Perf {
	perfMapMu.RLock()
	p := perfMap[goid()]
	perfMapMu.RUnlock()

	return p
}

// 供模板函数使用：取当前请求的统计对象（可能为 nil）
func CurrentPerf() *Perf {
	return currentPerf()
}

// 记录一条 SQL 及其耗时（由 ORM 日志 writer 调用）
func (p *Perf) addSQL(elapsed time.Duration) {
	if p == nil {
		return
	}

	atomic.AddInt64(&p.sqlNum, 1)
	atomic.AddInt64(&p.sqlTime, int64(elapsed))
}

// 服务端耗时
func (p *Perf) Elapsed() time.Duration {
	if p == nil {
		return 0
	}

	return time.Since(p.start)
}

// SQL 条数
func (p *Perf) SQLNum() int64 {
	if p == nil {
		return 0
	}

	return atomic.LoadInt64(&p.sqlNum)
}

// SQL 累计耗时
func (p *Perf) SQLTime() time.Duration {
	if p == nil {
		return 0
	}

	return time.Duration(atomic.LoadInt64(&p.sqlTime))
}

// 生成展示文本，例如：
//
//	本条 12.3ms ｜ SQL 8 次 3.1ms
//	本条 12.3ms                      （未开启 SQL 统计时）
func (p *Perf) Report() string {
	if p == nil {
		return ""
	}

	base := fmt.Sprintf("本条 %.1fms", float64(p.Elapsed().Nanoseconds())/1e6)

	if !sqlCountOn() {
		return base
	}

	return fmt.Sprintf("%s ｜ SQL %d 次 %.1fms",
		base, p.SQLNum(), float64(p.SQLTime().Nanoseconds())/1e6)
}

// 接入 ORM 日志，用于统计 SQL 条数与耗时
//
// beego ORM 只在 orm.Debug=true 时输出查询日志，因此开启 SQL 统计
// 时会同时打开该开关。日志内容被本 writer 消化（不打印），
// 以免污染应用日志。
//
// 返回一个「关闭」函数，用于恢复原状。
func HookORMLog() func() {
	// 保存原有设置
	oldLog := orm.DebugLog
	oldDebug := orm.Debug

	orm.DebugLog = orm.NewLog(perfWriter{})
	orm.Debug = true

	return func() {
		orm.DebugLog = oldLog
		orm.Debug = oldDebug
	}
}

// perfWriter 解析 ORM 日志并累加到对应请求
type perfWriter struct{}

func (w perfWriter) Write(b []byte) (int, error) {
	s := string(b)

	if strings.Contains(s, "Queries/") {
		if p := currentPerf(); p != nil {
			p.addSQL(parseORMElapsed(s))
		}
	}

	// 丢弃输出
	return len(b), nil
}

// 从 ORM 日志行中解析执行耗时
//
// 日志形如：
//
//	-[Queries/default] - [  OK /    db.Query /     0.5ms] - [SELECT ...]
//
// 解析失败返回 0（此时仍会计数，只是耗时为 0）。
func parseORMElapsed(line string) time.Duration {
	idx := strings.Index(line, "ms]")
	if idx < 0 {
		return 0
	}

	// 向前回溯数字与小
	start := idx
	for start > 0 {
		c := line[start-1]
		if (c >= '0' && c <= '9') || c == '.' {
			start--
			continue
		}
		break
	}

	if start == idx {
		return 0
	}

	ms, err := strconv.ParseFloat(line[start:idx], 64)
	if err != nil {
		return 0
	}

	return time.Duration(ms * float64(time.Millisecond))
}
