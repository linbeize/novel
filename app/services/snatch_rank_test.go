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

package services

import (
	"testing"
)

/*
起点排行榜采集测试

背景：原实现请求的 www.qidian.com/rank/...?style=1 已被 JS 挑战反爬拦截，
任何请求都只返回 HTTP 202 加一个 probe.js 空壳，且旧榜单路径多已 404。
现改用 m.qidian.com/rank/{name}/（服务端渲染，无需执行 JS）。

以下测试需要联网，且起点对频繁请求可能限流，故默认跳过。
显式开启：
    RUN_SNATCH_TEST=1 go test ./app/services/ -run TestSnatchRank -v
*/

// 各榜单应能取到书目
func TestSnatchRankQidianGetBooks(t *testing.T) {
	requireNetwork(t)

	r := NewSnatchRank()

	// 覆盖当前在用的全部榜单
	paths := []string{
		"hotsales", "yuepiao", "readindex", "newfans",
		"rec", "update", "sign", "newbook",
	}

	for _, p := range paths {
		books, err := r.getBooks(qidianMobileRankBase + "/" + p + "/")
		if err != nil {
			t.Errorf("[%s] 采集失败: %v", p, err)
			continue
		}
		if len(books) == 0 {
			t.Errorf("[%s] 未取到任何书目", p)
			continue
		}

		t.Logf("[%s] %d 本，前 3：%v", p, len(books), books[:min(3, len(books))])

		// 书名不应带站点后缀，否则无法与库中 name 精确匹配
		for _, b := range books {
			if b == "" {
				t.Errorf("[%s] 存在空书名", p)
				break
			}
			for _, suffix := range []string{"最新章节", "在线阅读", "无弹窗"} {
				if len(b) > len(suffix) && contains(b, suffix) {
					t.Errorf("[%s] 书名含站点后缀 %q: %q", p, suffix, b)
					break
				}
			}
		}
	}
}

// 书名必须干净：UpRecBatch 是按 name 精确匹配来标记推荐位的，
// 若带上「最新章节在线阅读」这类后缀则永远匹配不到任何一本。
func TestSnatchRankQidianBookNameClean(t *testing.T) {
	requireNetwork(t)

	r := NewSnatchRank()
	books, err := r.getBooks(qidianMobileRankBase + "/hotsales/")
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(books) == 0 {
		t.Fatal("未取到任何书目")
	}

	// 畅销榜通常包含知名作品，抽查书名形态
	for _, b := range books {
		if contains(b, "第") && contains(b, "章") {
			t.Errorf("书名疑似章节标题: %q", b)
		}
		if len([]rune(b)) > 40 {
			t.Errorf("书名异常长（%d 字）: %q", len([]rune(b)), b)
		}
	}

	t.Logf("畅销榜 %d 本，书名样例：%v", len(books), books[:min(5, len(books))])
}

// 去重：同一榜单内不应出现重复书名
func TestSnatchRankQidianDedup(t *testing.T) {
	requireNetwork(t)

	r := NewSnatchRank()
	books, err := r.getBooks(qidianMobileRankBase + "/readindex/")
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}

	seen := make(map[string]int)
	for _, b := range books {
		seen[b]++
	}

	for b, n := range seen {
		if n > 1 {
			t.Errorf("书名重复 %d 次: %q", n, b)
		}
	}

	t.Logf("阅读榜 %d 本，去重后 %d 本", len(books), len(seen))
}

// 小工具：避免为几行逻辑引入额外依赖
func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
