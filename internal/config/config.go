package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func LoadEnv(path string) error {
	h, _ := os.UserHomeDir()
	if h == "" {
		return godotenv.Load()
	}

	encKey := GetEncryptionKey(h)
	encData, err := os.ReadFile(path)
	if err == nil {
		dec := Decrypt(string(encData), encKey)
		if dec != nil {
			m, _ := godotenv.Unmarshal(string(dec))
			for k, v := range m {
				if v != "" {
					os.Setenv(k, v)
				}
			}
			return nil
		}
	}
	return godotenv.Load(path)
}

func GetEncryptionKey(home string) []byte {
	keyPath := filepath.Join(home, ".ghreport.key")
	keyData, err := os.ReadFile(keyPath)
	if err == nil && len(keyData) == 32 {
		return keyData
	}
	key := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, key)
	_ = os.WriteFile(keyPath, key, 0600)
	return key
}

func Encrypt(data []byte, key []byte) string {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func Decrypt(cryptoText string, key []byte) []byte {
	data, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil
	}
	return plaintext
}
