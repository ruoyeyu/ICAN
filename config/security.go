package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

func ValidateSecurityConfig() error {
	if len(Cfg.JWTSecret) < 32 || Cfg.JWTSecret == "infinite-canvas" {
		return errors.New("JWT_SECRET 必须配置为至少 32 字节的独立随机密钥")
	}
	u, err := url.Parse(Cfg.PublicOrigin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("PUBLIC_ORIGIN 必须为明确的 HTTP(S) 站点来源")
	}
	Cfg.PublicOrigin = strings.TrimRight(Cfg.PublicOrigin, "/")
	_, err = dataCipher()
	return err
}

func dataCipher() (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(Cfg.DataEncryptionKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("DATA_ENCRYPTION_KEY 必须是 32 字节随机密钥的标准 Base64 编码")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Associated data binds ciphertext to its owner and field; values cannot be
// transplanted between accounts. The encryption key is never persisted in DB.
func EncryptData(value, context string) (string, error) {
	if value == "" {
		return "", nil
	}
	aead, err := dataCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(value), []byte(context))
	return "ican:v1:" + base64.StdEncoding.EncodeToString(sealed), nil
}

func DecryptData(value, context string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "ican:v1:") {
		return "", errors.New("发现未迁移的明文敏感配置；请先备份并执行显式加密迁移")
	}
	aead, err := dataCipher()
	if err != nil {
		return "", err
	}
	sealed, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "ican:v1:"))
	if err != nil || len(sealed) < aead.NonceSize() {
		return "", errors.New("敏感配置密文无效")
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(context))
	if err != nil {
		return "", errors.New("敏感配置解密失败，请检查密钥与数据备份")
	}
	return string(plain), nil
}
