// Package token 生成并验证直链访问令牌（HMAC-SHA256）。
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// Generate 返回 token = HMAC-SHA256(secret, path|exp)，hex 编码（64 字符）。
func Generate(secret, path string, exp time.Time) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(path + "|" + strconv.FormatInt(exp.Unix(), 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Valid 校验 token 是否匹配且未过期，常数时间比较。
func Valid(secret, path, tok string, exp time.Time) bool {
	if tok == "" || time.Now().After(exp) {
		return false
	}
	expected := Generate(secret, path, exp)
	if len(tok) != len(expected) {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(tok))
}
