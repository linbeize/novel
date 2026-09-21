package log

import (
	"strings"
	"testing"
)

// 基本写入与读取
func TestRingAppendAndGet(t *testing.T) {
	Clear()

	Info("第一条")
	Warn("第二条")
	Error("第三条")

	entries := GetEntries("ALL", 0, 0)
	if len(entries) != 3 {
		t.Fatalf("应有 3 条，实际 %d", len(entries))
	}

	if entries[0].Message != "第一条" {
		t.Errorf("首条内容不符: %q", entries[0].Message)
	}
	if entries[2].Message != "第三条" {
		t.Errorf("末条内容不符: %q", entries[2].Message)
	}

	// 级别应被正确记录
	if entries[0].Level != "INFO" || entries[1].Level != "WARN" || entries[2].Level != "ERROR" {
		t.Errorf("级别记录不符: %s/%s/%s", entries[0].Level, entries[1].Level, entries[2].Level)
	}
}

// 级别过滤：应返回该级别及更严重的日志
func TestRingLevelFilter(t *testing.T) {
	Clear()

	Debug("调试")
	Info("信息")
	Warn("警告")
	Error("错误")

	cases := []struct {
		level string
		want  int
	}{
		{"ALL", 4},
		{"DEBUG", 4},
		{"INFO", 3},  // INFO/WARN/ERROR
		{"WARN", 2},  // WARN/ERROR
		{"ERROR", 1}, // 仅 ERROR
	}

	for _, c := range cases {
		got := GetEntries(c.level, 0, 0)
		if len(got) != c.want {
			t.Errorf("level=%s 应有 %d 条，实际 %d", c.level, c.want, len(got))
		}
	}
}

// 增量拉取：afterSeq 之前的记录应被跳过
func TestRingAfterSeq(t *testing.T) {
	Clear()

	Info("一")
	Info("二")

	all := GetEntries("ALL", 0, 0)
	if len(all) != 2 {
		t.Fatalf("应有 2 条，实际 %d", len(all))
	}

	mid := all[0].Seq

	// 只取 mid 之后的记录
	rest := GetEntries("ALL", 0, mid)
	if len(rest) != 1 {
		t.Fatalf("afterSeq=%d 应返回 1 条，实际 %d", mid, len(rest))
	}
	if rest[0].Message != "二" {
		t.Errorf("增量内容不符: %q", rest[0].Message)
	}

	// 传入最新序号时应无新记录
	latest := all[1].Seq
	if got := GetEntries("ALL", 0, latest); len(got) != 0 {
		t.Errorf("afterSeq 为最新序号时应无记录，实际 %d", len(got))
	}
}

// limit 限制：应返回最新的 N 条
func TestRingLimit(t *testing.T) {
	Clear()

	for i := 0; i < 10; i++ {
		Info("第", i, "条")
	}

	got := GetEntries("ALL", 3, 0)
	if len(got) != 3 {
		t.Fatalf("limit=3 应返回 3 条，实际 %d", len(got))
	}

	// 应是最后 3 条
	if !strings.Contains(got[2].Message, "9") {
		t.Errorf("limit 应保留最新记录，末条实际: %q", got[2].Message)
	}
}

// 容量上限：超出后应覆盖最旧的记录，且不超过容量
func TestRingCapacity(t *testing.T) {
	Clear()

	// 写入超过容量的条数
	total := ringCapacity + 50
	for i := 0; i < total; i++ {
		Info("x")
	}

	// 注意：GetEntries 的 limit 参数会截断结果，默认 500。
	// 这里要验证的是缓冲容量，故 limit 需传得比容量大。
	got := GetEntries("ALL", ringCapacity*2, 0)
	if len(got) != ringCapacity {
		t.Errorf("应有 %d 条（容量上限），实际 %d", ringCapacity, len(got))
	}

	// 未显式指定 limit 时应受默认值 500 限制
	if n := len(GetEntries("ALL", 0, 0)); n != 500 {
		t.Errorf("默认 limit 应为 500，实际返回 %d 条", n)
	}
}

// 清空
func TestRingClear(t *testing.T) {
	Clear()
	Info("内容")

	if len(GetEntries("ALL", 0, 0)) == 0 {
		t.Fatal("清空前应有记录")
	}

	Clear()

	if got := GetEntries("ALL", 0, 0); len(got) != 0 {
		t.Errorf("清空后应无记录，实际 %d", len(got))
	}
}

// 序号应单调递增，且 Clear 后不重置
//
// 不重置的原因：前端以 seq 作为游标，若重置会让旧日志被误判为新日志。
func TestRingSeqMonotonic(t *testing.T) {
	Clear()

	Info("a")
	first := Seq()

	Clear()

	Info("b")
	second := Seq()

	if second <= first {
		t.Errorf("Clear 后序号应继续递增：%d -> %d", first, second)
	}

	entries := GetEntries("ALL", 0, 0)
	if len(entries) != 1 {
		t.Fatalf("应有 1 条，实际 %d", len(entries))
	}
	if entries[0].Seq <= first {
		t.Errorf("新记录序号应大于旧的：%d <= %d", entries[0].Seq, first)
	}
}

// 多参数与多行文本的拼接
func TestFormat(t *testing.T) {
	Clear()

	// 中文冒号后不应补空格（若用 fmt.Sprintln 这里会多一个空格）
	Info("获取失败：", "timeout")
	e := GetEntries("ALL", 0, 0)
	if len(e) != 1 || e[0].Message != "获取失败：timeout" {
		t.Errorf("中文标点后不应补空格: %q", e[0].Message)
	}

	Clear()
	// 英文/数字之间应补空格，避免单词粘连
	Info("retry", 3, "times")
	e = GetEntries("ALL", 0, 0)
	if e[0].Message != "retry 3 times" {
		t.Errorf("英文参数间应补空格: %q", e[0].Message)
	}

	Clear()
	// 单个字符串参数应原样保留（含换行）
	Info("第一行\n第二行")
	e = GetEntries("ALL", 0, 0)
	if e[0].Message != "第一行\n第二行" {
		t.Errorf("单字符串不应被改动: %q", e[0].Message)
	}
}
