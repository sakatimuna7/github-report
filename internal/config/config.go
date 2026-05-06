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

// GetConfigBaseDir returns the standard application config directory.
// On Windows: %AppData%\ghreport
// On macOS: ~/Library/Application Support/ghreport
// On Linux: ~/.config/ghreport
func GetConfigBaseDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		h, _ := os.UserHomeDir()
		if h != "" {
			return filepath.Join(h, ".ghreport")
		}
		return ".ghreport" // Fallback to CWD
	}
	return filepath.Join(d, "ghreport")
}

// EnsureMigration checks for old config in home dir and migrates it to AppData if needed.
func EnsureMigration() error {
	newDir := GetConfigBaseDir()
	oldHome, _ := os.UserHomeDir()
	if oldHome == "" {
		return nil
	}

	// Migration map: old name -> new name
	filesToMigrate := map[string]string{
		".ghreport":                  "config",
		".ghreport.key":              "key",
		".ghreport_history.json":     "history.json",
		".ghreport_credentials.json": "google_credentials.json",
		".ghreport_token.json":       "google_token.json",
	}

	for oldName, newName := range filesToMigrate {
		oldPath := filepath.Join(oldHome, oldName)
		newPath := filepath.Join(newDir, newName)

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			if _, err := os.Stat(oldPath); err == nil {
				data, err := os.ReadFile(oldPath)
				if err == nil {
					_ = os.MkdirAll(newDir, 0755)
					_ = os.WriteFile(newPath, data, 0600)
				}
			}
		}
	}

	// Migrate templates directory
	oldTempDir := filepath.Join(oldHome, ".ghreport_templates")
	newTempDir := filepath.Join(newDir, "templates")
	if _, err := os.Stat(newTempDir); os.IsNotExist(err) {
		if _, err := os.Stat(oldTempDir); err == nil {
			_ = os.MkdirAll(newDir, 0755)
			_ = copyDir(oldTempDir, newTempDir)
		}
	}

	return nil
}

func copyDir(src string, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(dst, 0755)
	for _, entry := range entries {
		if entry.IsDir() {
			_ = copyDir(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()))
		} else {
			data, err := os.ReadFile(filepath.Join(src, entry.Name()))
			if err == nil {
				_ = os.WriteFile(filepath.Join(dst, entry.Name()), data, 0644)
			}
		}
	}
	return nil
}

func LoadEnv(path string) error {
	// Use the directory containing the config file for the encryption key
	base := filepath.Dir(path)
	encKey := GetEncryptionKey(base)
	
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

// SaveEnv encrypts and saves the configuration to the specified path.
func SaveEnv(path string, content string) error {
	base := filepath.Dir(path)
	if err := os.MkdirAll(base, 0755); err != nil {
		return err
	}
	
	encKey := GetEncryptionKey(base)
	encContent := Encrypt([]byte(content), encKey)
	
	return os.WriteFile(path, []byte(encContent), 0600)
}

func GetEncryptionKey(baseDir string) []byte {
	keyPath := filepath.Join(baseDir, "key")
	
	// Check if key exists in the new location
	keyData, err := os.ReadFile(keyPath)
	if err == nil && len(keyData) == 32 {
		return keyData
	}

	// Fallback check for legacy key in home dir (in case migration didn't run or failed)
	h, _ := os.UserHomeDir()
	if h != "" {
		legacyKeyPath := filepath.Join(h, ".ghreport.key")
		keyData, err = os.ReadFile(legacyKeyPath)
		if err == nil && len(keyData) == 32 {
			return keyData
		}
	}
	
	// Create new key if none found
	key := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, key)
	_ = os.MkdirAll(baseDir, 0755)
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
