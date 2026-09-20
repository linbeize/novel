package services

import (
	"testing"
)

// 分批入库的批次划分逻辑：验证「每 N 章落库一次 + 末尾余数也落库」
//
// 这里复刻采集循环中的批次判定，确保：
//   - 每累计 batchSize 章触发一次 flush
//   - 循环结束后若仍有剩余则再 flush 一次
//   - 任何情况下都不会遗漏章节
func batchSplit(total, batchSize int) []int {
	if batchSize < 1 {
		batchSize = DEFAULT_BATCH_SIZE
	}

	var flushes []int
	pending := 0
	for i := 0; i < total; i++ {
		pending++
		if pending >= batchSize {
			flushes = append(flushes, pending)
			pending = 0
		}
	}
	if pending > 0 {
		flushes = append(flushes, pending)
	}

	return flushes
}

func TestBatchSplit(t *testing.T) {
	cases := []struct {
		total, batch int
		want         []int
	}{
		// 正好整除
		{200, 100, []int{100, 100}},
		// 有余数 -> 末尾补一批
		{250, 100, []int{100, 100, 50}},
		// 不足一批
		{30, 100, []int{30}},
		// 单章
		{1, 100, []int{1}},
		// 空
		{0, 100, nil},
		// 每章一批
		{3, 1, []int{1, 1, 1}},
	}

	for _, c := range cases {
		got := batchSplit(c.total, c.batch)
		if len(got) != len(c.want) {
			t.Errorf("batchSplit(%d,%d) = %v, 期望 %v", c.total, c.batch, got, c.want)
			continue
		}
		sum := 0
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("batchSplit(%d,%d) = %v, 期望 %v", c.total, c.batch, got, c.want)
				break
			}
			sum += got[i]
		}
		// 关键性质：各批次之和必须等于总章节数（不丢章）
		if sum != c.total {
			t.Errorf("batchSplit(%d,%d) 批次合计 %d, 与总数不一致（会丢章）", c.total, c.batch, sum)
		}
	}
}

// 批次大小非法时回退到默认值
func TestBatchSizeFallback(t *testing.T) {
	got := batchSplit(150, 0)
	if len(got) != 2 || got[0] != 100 || got[1] != 50 {
		t.Errorf("batchSize=0 应回退到默认 %d，实际 %v", DEFAULT_BATCH_SIZE, got)
	}
}
