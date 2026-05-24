package sync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func EncryptField(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptField(encoded string, key []byte) (string, error) {
	if encoded == "" {
		return "", nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func EncryptConfigFiles(srcDir, destDir string, key []byte) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	if err := encryptConnectionsFile(
		filepath.Join(srcDir, "connections.json"),
		filepath.Join(destDir, "connections.json"),
		key,
	); err != nil {
		return fmt.Errorf("encrypt connections: %w", err)
	}
	if err := encryptAIConfigFile(
		filepath.Join(srcDir, "ai-config.json"),
		filepath.Join(destDir, "ai-config.json"),
		key,
	); err != nil {
		return fmt.Errorf("encrypt ai-config: %w", err)
	}
	return nil
}

func encryptConnectionsFile(src, dest string, key []byte) error {
	data, err := readJSONFile(src)
	if err != nil {
		return err
	}
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse connections: %w", err)
	}
	conns, _ := wrapper["connections"].([]interface{})
	for _, c := range conns {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if pw, ok := cm["password"].(string); ok && pw != "" {
			enc, err := EncryptField(pw, key)
			if err != nil {
				return err
			}
			cm["password"] = enc
		}
	}
	output, _ := json.MarshalIndent(wrapper, "", "  ")
	return os.WriteFile(dest, output, 0600)
}

func encryptAIConfigFile(src, dest string, key []byte) error {
	data, err := readJSONFile(src)
	if err != nil {
		return err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse ai-config: %w", err)
	}
	if apiKey, ok := config["apiKey"].(string); ok && apiKey != "" {
		enc, err := EncryptField(apiKey, key)
		if err != nil {
			return err
		}
		config["apiKey"] = enc
	}
	output, _ := json.MarshalIndent(config, "", "  ")
	return os.WriteFile(dest, output, 0600)
}

func DecryptConfigFiles(srcDir, destDir string, key []byte) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	if err := decryptConnectionsFile(
		filepath.Join(srcDir, "connections.json"),
		filepath.Join(destDir, "connections.json"),
		key,
	); err != nil {
		return fmt.Errorf("decrypt connections: %w", err)
	}
	if err := decryptAIConfigFile(
		filepath.Join(srcDir, "ai-config.json"),
		filepath.Join(destDir, "ai-config.json"),
		key,
	); err != nil {
		return fmt.Errorf("decrypt ai-config: %w", err)
	}
	return nil
}

func decryptConnectionsFile(src, dest string, key []byte) error {
	data, err := readJSONFile(src)
	if err != nil {
		return err
	}
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse connections: %w", err)
	}
	conns, _ := wrapper["connections"].([]interface{})
	for _, c := range conns {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if enc, ok := cm["password"].(string); ok && enc != "" {
			dec, err := DecryptField(enc, key)
			if err != nil {
				return err
			}
			cm["password"] = dec
		}
	}
	output, _ := json.MarshalIndent(wrapper, "", "  ")
	return os.WriteFile(dest, output, 0600)
}

func decryptAIConfigFile(src, dest string, key []byte) error {
	data, err := readJSONFile(src)
	if err != nil {
		return err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse ai-config: %w", err)
	}
	if enc, ok := config["apiKey"].(string); ok && enc != "" {
		dec, err := DecryptField(enc, key)
		if err != nil {
			return err
		}
		config["apiKey"] = dec
	}
	output, _ := json.MarshalIndent(config, "", "  ")
	return os.WriteFile(dest, output, 0600)
}

func readJSONFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte("{}"), nil
		}
		return nil, err
	}
	return data, nil
}
