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

package admin

import (
	"strconv"

	"github.com/vckai/novel/app/utils/log"
)

// 运行日志查看
//
// 采集任务在后台异步执行，其日志原本只能通过 journalctl 查看。
// 这里提供一个页面，直接读取内存中的最近日志（见 utils/log 的环形缓冲）。
type LogController struct {
	BaseController
}

// 日志页面
func (this *LogController) Index() {
	this.Data["Title"] = "运行日志"
	this.Data["Seq"] = log.Seq()

	this.View("log/index.tpl")
}

// 日志数据接口（JSON）
//
// 方法名不能叫 Data：嵌入的 BaseController 有 Data 字段，
// 同名方法会遮蔽该字段，导致函数内的 this.Data 无法索引。
//
// 参数：
//
//	level    级别过滤，ALL/DEBUG/INFO/WARN/ERROR，默认 ALL
//	limit    返回条数上限，默认 500，最大 2000
//	afterSeq 仅返回序号大于该值的记录，用于前端增量拉取
func (this *LogController) List() {
	level := this.GetString("level", "ALL")

	limit, _ := this.GetInt("limit", 500)
	if limit < 1 {
		limit = 500
	}
	if limit > 2000 {
		limit = 2000
	}

	seqStr := this.GetString("afterSeq")
	var afterSeq uint64
	if seqStr != "" {
		if v, err := strconv.ParseUint(seqStr, 10, 64); err == nil {
			afterSeq = v
		}
	}

	entries := log.GetEntries(level, limit, afterSeq)

	// 同时返回最新序号，前端据此推进游标
	this.OutJson(0, "", map[string]interface{}{
		"seq":     log.Seq(),
		"entries": entries,
	})
}

// 清空内存中的日志缓冲
//
// 仅清空页面展示用的缓冲，不影响 journalctl 可查的历史日志。
func (this *LogController) Clear() {
	log.Clear()

	this.OutJson(0, "已清空", nil)
}
