package bqglll

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

/*
bqglll.cc 接口签名（token 生成）
=================================

该站正文接口 /api/chapter 强制校验 token，token 是对请求参数做
AES-128-CBC 加密后再 base64 得到的：

	code  = MD5("book" + "@" + "token" + "." + "html")
	      = MD5("book@token.html")
	iv    = code[0:16]     // 前 16 个十六进制字符，当作 ASCII 字节
	key   = code[16:32]    // 后 16 个
	token = base64( AES-CBC-PKCS7( JSON(params), key, iv ) )
	url   = /api/chapter?token=<urlencode(token)>

要点说明：

  - key/iv 取自同一个 MD5 字符串的前后半段，各 16 字节（AES-128）。
    这里按 ASCII 字节处理，而非把十六进制再解码成 8 字节——
    与站点前端 CryptoJS 的行为一致（其 key/iv 均是字符串）。

  - 密钥由固定常量派生，不含时间戳或 nonce，因此同一组参数生成的
    token 恒定、可重放。若站点更换密钥，只需调整下面的 salt/path 常量，
    无需改动其它代码。

  - 参数既可放在 token 内，也可作为普通 query 传递。
    据实测，/api/book 以 query 参数优先；正文接口则只认 token。
*/

const (
	// 密钥派生常量。站点混淆代码中拆散在多个字符串表项里，
	// 拼回后即为 "book" + "@" + "token" + "." + "html"。
	tokenSalt = "book"
	tokenJoin = "@"
	tokenName = "token"
	tokenExt  = "." + "html"
)

// ChapterParams 章节接口的参数
//
// 用结构体而非 map：Go 的 map 序列化会按字段名字母序输出
// （chapterid 会排到 id 前面），而站点的前端是按书写顺序序列化的。
// 由于加密的是字节流，顺序不同会导致密文不同、校验失败，
// 因此这里用结构体固定字段顺序，与站点保持一致。
type ChapterParams struct {
	ID        uint32 `json:"id"`
	ChapterID uint32 `json:"chapterid"`
}

// EncryptToken 把参数加密为可用的 token（已做 URL 编码）
//
// params 会被序列化为 JSON，故字段名与顺序需与站点接口一致。
func EncryptToken(params interface{}) (string, error) {
	plain, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("参数序列化失败: %w", err)
	}

	key, iv, err := deriveKey()
	if err != nil {
		return "", err
	}

	enc, err := aesEncryptCBC(plain, key, iv)
	if err != nil {
		return "", err
	}

	return urlEncode(base64.StdEncoding.EncodeToString(enc)), nil
}

// deriveKey 从固定常量派生 key 与 iv
func deriveKey() (key, iv []byte, err error) {
	raw := tokenSalt + tokenJoin + tokenName + tokenExt

	sum := md5.Sum([]byte(raw))
	code := hex.EncodeToString(sum[:]) // 32 个字符

	// 前 16 字符作 iv、后 16 字符作 key（按 ASCII 字节，与前端一致）
	iv = []byte(code[0:16])
	key = []byte(code[16:32])

	return key, iv, nil
}

// aesEncryptCBC AES-128-CBC + PKCS7 填充
func aesEncryptCBC(plain, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES 初始化失败: %w", err)
	}

	// PKCS7：补齐到块大小整数倍
	padded := pkcs7Pad(plain, block.BlockSize())

	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)

	return out, nil
}

// pkcs7Pad PKCS7 填充
//
// 注意：当明文长度正好是块大小整数倍时，需补满一整块，
// 否则解密端无法判断末尾是数据还是填充。
func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

// urlEncode 仅对 token 中会影响 query 解析的字符做编码
//
// 与 JS 的 encodeURIComponent 保持一致：base64 的 + / = 需要转义，
// 字母数字与 - _ . ! ~ * ' ( ) 保持不变。
func urlEncode(s string) string {
	const hexChars = "0123456789ABCDEF"

	var b bytes.Buffer
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '!' ||
			c == '~' || c == '*' || c == '\'' || c == '(' || c == ')' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexChars[c>>4])
		b.WriteByte(hexChars[c&0x0F])
	}

	return b.String()
}
