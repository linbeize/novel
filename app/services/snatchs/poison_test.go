package snatchs

import (
	"testing"
)

// 投毒检测：真实样本比对
//
// 样本取自 2026-09 对 bqglll.cc 的实测记录。
func TestDetectPoison(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "投毒-特征开头",
			content: "我今年２２岁，是一个大学阿拉伯语专业的学生，全班一共３０人，有２５人是女生bqg82○ de另４个男生长的委琐不堪",
			want:    true,
		},
		{
			name:    "投毒-含HTML标签",
			content: "<br/>　　我今年２２岁，是一个大学阿拉伯语专业的学生，全班一共３０人<br/>　　而我是典型东北大汉的身材",
			want:    true,
		},
		{
			name:    "投毒-水印+成人词",
			content: "她有些惊慌：你怎麽可以？<br/>我伸出双手，抓向了她的乳房caxao♀com<br/>",
			want:    true,
		},
		{
			name:    "投毒-半角写法",
			content: "我今年22岁，是一个大学阿拉伯语专业的学生，全班一共30人",
			want:    true,
		},
		{
			name:    "正常-机武风暴第一章",
			content: "当<br/>当<br/>当<br/>“来自太阳系联盟的观众们，S10泛太阳系机武大赛圆满落下帷幕，让我们恭喜NUP月球联邦的勇士们",
			want:    false,
		},
		{
			name:    "正常-斗将行",
			content: "地窟中。<br/>众黑袍人有些疑惑。<br/>摄魂神光既出，就该将聚灵大阵中散魂乱魄与战魂一并摄入都天万鬼旗",
			want:    false,
		},
		{
			name:    "正常-含数字与英文",
			content: "李昊看了一下系统面板，EMP3000分，完成任务需要六个月。他打开机甲的自动门，进入了驾驶舱",
			want:    false,
		},
		{
			name:    "正常-占位文案",
			content: "本章由于字数太少，暂不显示。如果你觉得本章比较重要，可以选择左下方报错章节",
			want:    false,
		},
		{
			name:    "空内容",
			content: "",
			want:    false,
		},
		{
			name:    "正常-含英文单词与点号（不应误判为水印）",
			content: "这是一个 test.com 域名的例子，小说里提到 https://www.example.com 这样的网址也很常见",
			want:    true, // 域名形态会命中水印规则，但需成人词才判定；此处无成人词 → 见下方断言
		},
	}

	for _, c := range cases {
		got := DetectPoison(c.content)
		// 最后一例用于验证「仅水印不足以判定」
		if c.name == "正常-含英文单词与点号（不应误判为水印）" {
			if got.IsPoison {
				t.Errorf("  %s: 误判为投毒（仅含水印无成人词）", c.name)
			} else {
				t.Logf("  ✅ %s: 正确未误判", c.name)
			}
			continue
		}

		if got.IsPoison != c.want {
			t.Errorf("  %s: 期望 %v，实际 %v（%s）", c.name, c.want, got.IsPoison, got.Reason)
		} else {
			t.Logf("  ✅ %s → %v %s", c.name, got.IsPoison, got.Reason)
		}
	}
}

// 连续命中计数与阈值
func TestPoisonTracker(t *testing.T) {
	src := "test-source"

	// 前两次不应触发阈值
	for i := 1; i <= poisonThreshold-1; i++ {
		hit := PoisonObserved(src, PoisonResult{true, "test"}, "sample")
		if hit {
			t.Errorf("  第 %d 次就触发阈值（阈值为 %d）", i, poisonThreshold)
		}
	}

	// 第三次达到阈值
	hit := PoisonObserved(src, PoisonResult{true, "test"}, "sample")
	if !hit {
		t.Errorf("  第 %d 次未触发阈值", poisonThreshold)
	}
	t.Logf("  ✅ 连续命中 %d 次触发阈值", poisonThreshold)

	st := GetPoisonStatus(src)
	if st.Streak != poisonThreshold {
		t.Errorf("  状态计数不符: %d", st.Streak)
	}
	if st.LastAt.IsZero() {
		t.Errorf("  未记录命中时间")
	}
	t.Logf("  ✅ 状态: streak=%d at=%s reason=%s",
		st.Streak, st.LastAt.Format("2006-01-02 15:04:05"), st.Reason)

	// 正常内容应重置计数
	PoisonCleared(src)
	st = GetPoisonStatus(src)
	if st.Streak != 0 {
		t.Errorf("  重置后计数应为 0，实际 %d", st.Streak)
	}
	t.Logf("  ✅ 正常内容后计数已重置")
}

// 多源互不干扰
func TestPoisonTrackerIsolation(t *testing.T) {
	PoisonObserved("src-a", PoisonResult{true, "a"}, "x")
	PoisonObserved("src-a", PoisonResult{true, "a"}, "x")

	stA := GetPoisonStatus("src-a")
	stB := GetPoisonStatus("src-b")

	if stA.Streak != 2 {
		t.Errorf("  src-a 计数应为 2，实际 %d", stA.Streak)
	}
	if stB.Streak != 0 {
		t.Errorf("  src-b 不应受影响，实际 %d", stB.Streak)
	}
	t.Logf("  ✅ 多源计数独立")

	all := AllPoisonStatus()
	if len(all) != 1 {
		t.Errorf("  应只有 1 个源有记录，实际 %d", len(all))
	}
}
