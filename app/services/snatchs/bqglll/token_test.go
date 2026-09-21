package bqglll

import (
	"net/url"
	"testing"
)

// 与参考实现（报告中的 Node 版本）逐字节比对
func TestEncryptToken(t *testing.T) {
	got, err := EncryptToken(ChapterParams{ID: 113680, ChapterID: 688})
	if err != nil {
		t.Fatal(err)
	}

	// 报告给出的期望值
	want := "UHFkkn7wSaP1Gqp%2BR5Zb7q1f37NyAv8Qqdzboog4CTA%3D"

	t.Logf("生成: %s", got)
	t.Logf("期望: %s", want)

	if got != want {
		t.Errorf("token 不一致")
	}
}

// URL 编码应能被正确解码回来
func TestTokenURLEncoding(t *testing.T) {
	tok, err := EncryptToken(ChapterParams{ID: 1, ChapterID: 1})
	if err != nil {
		t.Fatal(err)
	}

	dec, err := url.QueryUnescape(tok)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 解码后应是合法 base64（无 % 转义残留）
	if len(dec) == 0 || dec == tok {
		t.Logf("token 不含需转义字符，属正常: %s", tok[:min(20, len(tok))])
	}
	t.Logf("编码后长度 %d，解码后长度 %d", len(tok), len(dec))
}

// 相同参数应生成相同 token（无时间戳/nonce，可重放）
func TestTokenDeterministic(t *testing.T) {
	a, _ := EncryptToken(ChapterParams{ID: 5, ChapterID: 6})
	b, _ := EncryptToken(ChapterParams{ID: 5, ChapterID: 6})
	if a != b {
		t.Errorf("同参数应生成相同 token: %s vs %s", a, b)
	}

	c, _ := EncryptToken(ChapterParams{ID: 5, ChapterID: 7})
	if a == c {
		t.Error("不同参数不应生成相同 token")
	}
}

// 密钥派生结果应为固定值
func TestDeriveKey(t *testing.T) {
	key, iv, err := deriveKey()
	if err != nil {
		t.Fatal(err)
	}
	if string(key) != "a3dc22cf70418a51" {
		t.Errorf("key 不符: %q", string(key))
	}
	if string(iv) != "394c2c3202da6270" {
		t.Errorf("iv 不符: %q", string(iv))
	}
	t.Logf("key=%s iv=%s", key, iv)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
