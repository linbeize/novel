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
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 采集节流器
//
// 背景：原实现没有任何全局节流，采集 worker 会并发请求同一站点。
// 实测日志中「同一秒内最多出现 15 个请求」打向同一域名，随即被目标站
// 返回 502 限流；而失败重试只在循环里固定 sleep 10ms，等于被限流时继续
// 猛打，形成重试风暴（单个 URL 曾被重复请求 16 次）。
//
// 本组件提供两个能力：
//  1. 同域名串行：同一时刻每个域名只放行一个请求，从根本上消除并发冲击；
//  2. 最小请求间隔 + 随机抖动：用时间换稳定，避免固定节奏被识别为爬虫。
//
// 锁的分工（避免数据竞争）：
//   - h.mu      : 串行锁，请求期间持有，保证同域名同一时刻只有一个请求
//   - this.mu   : 保护 lastHit / cooldownUntil 等字段的读写
type Throttle struct {
	mu    sync.Mutex
	hosts map[string]*hostState
}

type hostState struct {
	// 串行锁：Wait 获取、Done 释放
	mu sync.Mutex

	// 以下字段由 Throttle.mu 保护
	lastHit       time.Time
	cooldownUntil time.Time
}

var (
	defaultThrottle *Throttle
	throttleOnce    sync.Once
)

// 获取全局节流器（单例，所有采集路径共用）
func ThrottleInstance() *Throttle {
	throttleOnce.Do(func() {
		defaultThrottle = &Throttle{
			hosts: make(map[string]*hostState),
		}
	})

	return defaultThrottle
}

// 从 URL 中提取主机名作为限流键
func hostKey(rawurl string) string {
	u, err := url.Parse(strings.TrimSpace(rawurl))
	if err != nil || u.Hostname() == "" {
		return rawurl
	}

	return strings.ToLower(u.Hostname())
}

// 取得某个域名的状态（不存在则创建）
func (this *Throttle) host(rawurl string) *hostState {
	key := hostKey(rawurl)

	this.mu.Lock()
	defer this.mu.Unlock()

	h, ok := this.hosts[key]
	if !ok {
		h = &hostState{}
		this.hosts[key] = h
	}

	return h
}

// 读取冷却与上次请求时间
func (this *Throttle) state(h *hostState) (lastHit, cooldownUntil time.Time) {
	this.mu.Lock()
	defer this.mu.Unlock()

	return h.lastHit, h.cooldownUntil
}

// 等待直到可以发起下一次请求
//
// 调用方必须在请求结束后调用 Done，否则该域名会被一直占用。
// interval: 两次请求间最小间隔；jitter: 额外随机抖动上限。
func (this *Throttle) Wait(rawurl string, interval, jitter time.Duration) {
	h := this.host(rawurl)

	// 阻塞式获取域名串行锁 -> 同域名同一时刻只有一个请求在途
	h.mu.Lock()

	// 若处于冷却期，先等到冷却结束
	if _, until := this.state(h); !until.IsZero() {
		if d := time.Until(until); d > 0 {
			time.Sleep(d)
		}
	}

	// 满足最小间隔
	if interval > 0 {
		if last, _ := this.state(h); !last.IsZero() {
			if d := interval - time.Since(last); d > 0 {
				time.Sleep(d)
			}
		}
	}

	// 随机抖动，打散固定节奏
	if jitter > 0 {
		time.Sleep(time.Duration(rand.Int63n(int64(jitter))))
	}
}

// 请求结束：记录时间并释放域名锁。成功与失败都必须调用。
func (this *Throttle) Done(rawurl string) {
	h := this.host(rawurl)

	this.mu.Lock()
	h.lastHit = time.Now()
	this.mu.Unlock()

	h.mu.Unlock()
}

// 触发限流后的冷却：让该域名在一段时间内不再发起任何请求。
//
// 与逐请求退避不同，这作用于整个站点 —— 一旦确认被限流，
// 短期完全停手，等待对方恢复，避免持续触发封禁。
func (this *Throttle) Cooldown(rawurl string, d time.Duration) {
	h := this.host(rawurl)

	this.mu.Lock()
	defer this.mu.Unlock()

	if until := time.Now().Add(d); until.After(h.cooldownUntil) {
		h.cooldownUntil = until
	}
}
