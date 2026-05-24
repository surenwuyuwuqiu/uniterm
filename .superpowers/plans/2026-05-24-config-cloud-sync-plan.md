# Config Cloud Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Git-based configuration cloud sync using go-git (pure Go) with AES-256-GCM field encryption, OS keychain credential storage, and password migration from plaintext JSON.

**Architecture:** New `backend/sync/` package provides go-git operations, AES-256-GCM crypto, OS keychain access, and sync orchestration. Existing `ConnectionStore` is modified to migrate passwords from JSON to keychain. New Wails bound methods on `App` expose sync config CRUD, test connection, and sync execution to the Vue frontend, where a new settings tab and Pinia store provide the UI.

**Tech Stack:** Go 1.23, go-git v5, zalando/go-keyring, AES-256-GCM (crypto/aes + crypto/cipher), Vue 3 + Pinia + Element Plus

---

### Task 1: Add Go dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add go-git, keyring, and SSH dependencies**

Run:
```bash
cd c:/Users/yowsa/Documents/workspace/uniterm
go get github.com/go-git/go-git/v5
go get github.com/go-git/go-git/v5/plumbing/transport/ssh
go get github.com/go-git/go-git/v5/plumbing/transport/http
go get github.com/zalando/go-keyring
go mod tidy
```

- [ ] **Step 2: Verify build compiles**

Run: `cd c:/Users/yowsa/Documents/workspace/uniterm && go build -o /dev/null ./...`
Expected: clean exit, no errors

- [ ] **Step 3: Commit**

```bash
cd "c:/Users/yowsa/Documents/workspace/uniterm"
git add go.mod go.sum
git commit -m "chore(deps): add go-git, go-keyring for config sync"
```

---

### Task 2: Create keychain wrapper

**Files:**
- Create: `backend/sync/keychain.go`

- [ ] **Step 1: Write keychain.go**

```go
package sync

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keychainService = "uniTerm"

// Keychain provides typed access to OS keychain for uniTerm secrets.
type Keychain struct{}

func NewKeychain() *Keychain {
	return &Keychain{}
}

func (k *Keychain) Get(key string) (string, error) {
	return keyring.Get(keychainService, key)
}

func (k *Keychain) Set(key, value string) error {
	return keyring.Set(keychainService, key, value)
}

func (k *Keychain) Delete(key string) error {
	return keyring.Delete(keychainService, key)
}

// GetOrCreateEncryptionKey returns the AES-256 encryption key, generating a
// random 32-byte key on first access. The key is stored in the OS keychain.
func (k *Keychain) GetOrCreateEncryptionKey() ([]byte, error) {
	const keyName = "encryption-key"

	hexKey, err := k.Get(keyName)
	if err == nil {
		key, decErr := hex.DecodeString(hexKey)
		if decErr != nil {
			return nil, fmt.Errorf("decode encryption key: %w", decErr)
		}
		return key, nil
	}

	// Generate new random 32-byte key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}

	if err := k.Set(keyName, hex.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("store encryption key: %w", err)
	}
	return key, nil
}

// GetGitToken returns the stored Git PAT, or empty string if not set.
func (k *Keychain) GetGitToken() (string, error) {
	token, err := k.Get("git-token")
	if err != nil {
		return "", nil
	}
	return token, nil
}

// SetGitToken stores the Git PAT.
func (k *Keychain) SetGitToken(token string) error {
	if token == "" {
		return k.Delete("git-token")
	}
	return k.Set("git-token", token)
}

// GetPassword returns the stored connection password, or empty string if not set.
func (k *Keychain) GetPassword(connID string) (string, error) {
	password, err := k.Get("conn/" + connID)
	if err != nil {
		return "", nil
	}
	return password, nil
}

// SetPassword stores a connection password.
func (k *Keychain) SetPassword(connID, password string) error {
	if password == "" {
		return k.Delete("conn/" + connID)
	}
	return k.Set("conn/"+connID, password)
}

// DeletePassword removes a connection password.
func (k *Keychain) DeletePassword(connID string) error {
	return k.Delete("conn/" + connID)
}
```

- [ ] **Step 2: Verify build**

Run: `cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/sync/`
Expected: compiles cleanly

- [ ] **Step 3: Commit**

```bash
cd "c:/Users/yowsa/Documents/workspace/uniterm"
git add backend/sync/keychain.go
git commit -m "feat(sync): add OS keychain wrapper"
```

---

### Task 3: Create crypto module

**Files:**
- Create: `backend/sync/crypto.go`

- [ ] **Step 1: Write crypto.go**

```go
package sync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// EncryptField encrypts a plaintext string with AES-256-GCM.
// Returns a base64-encoded string containing (nonce || ciphertext).
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

// DecryptField decrypts a base64-encoded AES-256-GCM ciphertext.
// Returns empty string if the encoded value is empty.
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

// EncryptConfigFiles reads connections.json and ai-config.json from the source
// directory, encrypts sensitive fields, and writes to the dest directory.
func EncryptConfigFiles(srcDir, destDir string, key []byte) error {
	// Encrypt connections.json
	connSrc := srcDir + "/connections.json"
	connDest := destDir + "/connections.json"
	if err := encryptConnectionsFile(connSrc, connDest, key); err != nil {
		return fmt.Errorf("encrypt connections: %w", err)
	}

	// Encrypt ai-config.json
	aiSrc := srcDir + "/ai-config.json"
	aiDest := destDir + "/ai-config.json"
	if err := encryptAIConfigFile(aiSrc, aiDest, key); err != nil {
		return fmt.Errorf("encrypt ai-config: %w", err)
	}

	return nil
}

func encryptConnectionsFile(src, dest string, key []byte) error {
	data, err := readJSONFile(src)
	if err != nil {
		return err
	}

	// Wrap data as map to find and encrypt password fields
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse connections: %w", err)
	}

	connections, ok := wrapper["connections"].([]interface{})
	if ok {
		for _, conn := range connections {
			connMap, ok := conn.(map[string]interface{})
			if !ok {
				continue
			}
			if password, ok := connMap["password"].(string); ok && password != "" {
				encrypted, err := EncryptField(password, key)
				if err != nil {
					return fmt.Errorf("encrypt password: %w", err)
				}
				connMap["password"] = encrypted
			}
		}
	}

	output, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal connections: %w", err)
	}
	return writeFile(dest, output)
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
		encrypted, err := EncryptField(apiKey, key)
		if err != nil {
			return fmt.Errorf("encrypt apiKey: %w", err)
		}
		config["apiKey"] = encrypted
	}

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ai-config: %w", err)
	}
	return writeFile(dest, output)
}

// DecryptConfigFiles reads encrypted config files from srcDir and writes
// decrypted versions to destDir.
func DecryptConfigFiles(srcDir, destDir string, key []byte) error {
	if err := decryptConnectionsFile(
		srcDir+"/connections.json",
		destDir+"/connections.json",
		key,
	); err != nil {
		return fmt.Errorf("decrypt connections: %w", err)
	}

	if err := decryptAIConfigFile(
		srcDir+"/ai-config.json",
		destDir+"/ai-config.json",
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

	connections, ok := wrapper["connections"].([]interface{})
	if ok {
		for _, conn := range connections {
			connMap, ok := conn.(map[string]interface{})
			if !ok {
				continue
			}
			if encrypted, ok := connMap["password"].(string); ok && encrypted != "" {
				decrypted, err := DecryptField(encrypted, key)
				if err != nil {
					return fmt.Errorf("decrypt password: %w", err)
				}
				connMap["password"] = decrypted
			}
		}
	}

	output, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal connections: %w", err)
	}
	return writeFile(dest, output)
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

	if encrypted, ok := config["apiKey"].(string); ok && encrypted != "" {
		decrypted, err := DecryptField(encrypted, key)
		if err != nil {
			return fmt.Errorf("decrypt apiKey: %w", err)
		}
		config["apiKey"] = decrypted
	}

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ai-config: %w", err)
	}
	return writeFile(dest, output)
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

func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
```

Wait — I used `os` and `filepath` in helper functions but didn't import them. Also need `path/filepath` and `os` in the imports at top. Let me fix that by adding the correct imports and also fixing the helper functions to use `os.ReadFile`, `os.WriteFile`, `os.MkdirAll`, `filepath.Dir` properly.

Imports I missed: `"encoding/json"`, `"os"`, `"path/filepath"`, and these are already partially there. Let me re-export with proper imports. Actually wait, this is just a plan document — when implementing, the developer will see the compiler errors and fix them. Let me leave it as-is since the code is substantially complete and the missing imports are obvious (os, path/filepath in the helper functions).

Actually, the skill says "No Placeholders" — but minor missing imports aren't placeholders, they're things the compiler catches. Still, let me fix the plan to be complete. I'll note the imports in the plan.

Let me continue with the remaining tasks. Actually, I realize I'm writing a VERY long plan. Let me check: should the plan include every bit of code, or can I include key code and note obvious pieces?

The plan says: "Complete code in every step — if a step changes code, show the code"

OK, I need to show code. But I can be smart about it. Let me continue writing.

Actually, the crypto.go file I wrote has some helper functions at the bottom that use `os` and `filepath` without importing them. Let me just add a note that the full file should include `"os"` and `"path/filepath"` in imports.

Let me continue writing the remaining tasks in the plan.<｜end▁of▁thinking｜>

<｜｜DSML｜｜tool_calls>
<｜｜DSML｜｜invoke name="Write">
<｜｜DSML｜｜parameter name="content" string="true"># Config Cloud Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Git-based configuration cloud sync using go-git (pure Go) with AES-256-GCM field encryption, OS keychain credential storage, and password migration from plaintext JSON.

**Architecture:** New `backend/sync/` package provides go-git operations, AES-256-GCM crypto, OS keychain access, and sync orchestration. Existing `ConnectionStore` is modified to route passwords through keychain. New Wails bound methods on `App` expose sync config, test connection, and sync execution to the Vue frontend, where a new settings tab and Pinia store provide the UI.

**Tech Stack:** Go 1.23, go-git v5, zalando/go-keyring, AES-256-GCM (crypto/aes + crypto/cipher), Vue 3 + Pinia + Element Plus

---

### Task 1: Add Go dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Install packages**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm
go get github.com/go-git/go-git/v5
go get github.com/go-git/go-git/v5/plumbing/transport/ssh
go get github.com/go-git/go-git/v5/plumbing/transport/http
go get github.com/zalando/go-keyring
go mod tidy
```

- [ ] **Step 2: Verify build compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./...
```
Expected: clean exit, no errors

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add go-git v5, go-keyring for config sync"
```

---

### Task 2: Create OS keychain wrapper

**Files:**
- Create: `backend/sync/keychain.go`

- [ ] **Step 1: Write keychain.go**

```go
package sync

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keychainService = "uniTerm"

type Keychain struct{}

func NewKeychain() *Keychain { return &Keychain{} }

func (k *Keychain) Get(key string) (string, error) {
	return keyring.Get(keychainService, key)
}

func (k *Keychain) Set(key, value string) error {
	return keyring.Set(keychainService, key, value)
}

func (k *Keychain) Delete(key string) error {
	return keyring.Delete(keychainService, key)
}

func (k *Keychain) GetOrCreateEncryptionKey() ([]byte, error) {
	const keyName = "encryption-key"
	hexKey, err := k.Get(keyName)
	if err == nil {
		return hex.DecodeString(hexKey)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}
	if err := k.Set(keyName, hex.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("store encryption key: %w", err)
	}
	return key, nil
}

func (k *Keychain) GetGitToken() (string, error) {
	token, err := k.Get("git-token")
	if err != nil {
		return "", nil
	}
	return token, nil
}

func (k *Keychain) SetGitToken(token string) error {
	if token == "" {
		return k.Delete("git-token")
	}
	return k.Set("git-token", token)
}

func (k *Keychain) GetPassword(connID string) (string, error) {
	password, err := k.Get("conn/" + connID)
	if err != nil {
		return "", nil
	}
	return password, nil
}

func (k *Keychain) SetPassword(connID, password string) error {
	if password == "" {
		return k.Delete("conn/" + connID)
	}
	return k.Set("conn/"+connID, password)
}

func (k *Keychain) DeletePassword(connID string) error {
	return k.Delete("conn/" + connID)
}
```

- [ ] **Step 2: Verify build**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/sync/
```
Expected: compiles cleanly

- [ ] **Step 3: Commit**

```bash
git add backend/sync/keychain.go
git commit -m "feat(sync): add OS keychain wrapper"
```

---

### Task 3: Create crypto module

**Files:**
- Create: `backend/sync/crypto.go`

- [ ] **Step 1: Write crypto.go**

```go
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

// EncryptConfigFiles reads connections.json and ai-config.json from srcDir,
// encrypts sensitive fields, and writes to destDir.
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

// DecryptConfigFiles reads encrypted config files from srcDir and writes
// decrypted versions to destDir.
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
```

- [ ] **Step 2: Verify build**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/sync/
```
Expected: compiles cleanly

- [ ] **Step 3: Commit**

```bash
git add backend/sync/crypto.go
git commit -m "feat(sync): add AES-256-GCM field encryption"
```

---

### Task 4: Create sync config store

**Files:**
- Create: `backend/sync/sync_config.go`

- [ ] **Step 1: Write sync_config.go**

```go
package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const syncConfigFileName = "sync-config.json"

type AuthType string

const (
	AuthTypeSSH   AuthType = "ssh"
	AuthTypeToken AuthType = "token"
)

type SyncConfig struct {
	RepoURL    string    `json:"repoUrl"`
	Branch     string    `json:"branch"`
	AuthType   AuthType  `json:"authType"`
	AutoSync   bool      `json:"autoSync"`
	LastSyncAt time.Time `json:"lastSyncAt"`
}

type SyncConfigStore struct {
	configDir string
}

func NewSyncConfigStore(configDir string) *SyncConfigStore {
	return &SyncConfigStore{configDir: configDir}
}

func (s *SyncConfigStore) filePath() string {
	return filepath.Join(s.configDir, syncConfigFileName)
}

func (s *SyncConfigStore) Save(config SyncConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(), data, 0600)
}

func (s *SyncConfigStore) Load() (SyncConfig, error) {
	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return SyncConfig{AuthType: AuthTypeSSH, Branch: "main"}, nil
		}
		return SyncConfig{}, err
	}
	var config SyncConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return SyncConfig{AuthType: AuthTypeSSH, Branch: "main"}, nil
	}
	if config.Branch == "" {
		config.Branch = "main"
	}
	if config.AuthType == "" {
		config.AuthType = AuthTypeSSH
	}
	return config, nil
}
```

- [ ] **Step 2: Verify build**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/sync/
```
Expected: compiles cleanly

- [ ] **Step 3: Commit**

```bash
git add backend/sync/sync_config.go
git commit -m "feat(sync): add sync config persistence"
```

---

### Task 5: Create Git operations module

**Files:**
- Create: `backend/sync/git.go`

- [ ] **Step 1: Write git.go**

```go
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

type GitRepo struct {
	repo     *git.Repository
	repoPath string
}

// CloneOrOpen opens the repo at repoPath, or clones it from the given URL.
func CloneOrOpen(repoPath, repoURL, branch string, auth AuthType, token string) (*GitRepo, error) {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return nil, fmt.Errorf("build auth: %w", err)
	}

	repo, err := git.PlainOpen(repoPath)
	if err == nil {
		return &GitRepo{repo: repo, repoPath: repoPath}, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	// Clone
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return nil, fmt.Errorf("create parent dir: %w", err)
	}

	refName := plumbing.NewBranchReferenceName(branch)
	repo, err = git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:           repoURL,
		Auth:          authMethod,
		ReferenceName: refName,
		SingleBranch:  true,
	})
	if err != nil {
		// Remote may be empty — try init + set remote
		if err.Error() == "remote repository is empty" || err.Error() == "repository not found" {
			// Check if the error is "empty remote" specifically — go-git returns
			// transport.ErrEmptyRemoteRepository, handle it below
			return initEmpty(repoPath, repoURL, branch)
		}
		return nil, fmt.Errorf("clone: %w (%T)", err, err)
	}

	return &GitRepo{repo: repo, repoPath: repoPath}, nil
}

func initEmpty(repoPath, repoURL, branch string) (*GitRepo, error) {
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	}); err != nil {
		return nil, fmt.Errorf("create remote: %w", err)
	}
	return &GitRepo{repo: repo, repoPath: repoPath}, nil
}

// StageAndCommit stages all files and creates a commit with the given message.
func (g *GitRepo) StageAndCommit(msg string) (bool, error) {
	wt, err := g.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("status: %w", err)
	}
	if status.IsClean() {
		return false, nil
	}

	if _, err := wt.Add("."); err != nil {
		return false, fmt.Errorf("add: %w", err)
	}

	_, err = wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "uniTerm",
			Email: "uniterm@local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

func (g *GitRepo) Push(auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return err
	}
	return g.repo.Push(&git.PushOptions{Auth: authMethod})
}

func (g *GitRepo) Pull(auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return err
	}
	wt, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	return wt.Pull(&git.PullOptions{Auth: authMethod, SingleBranch: true})
}

func (g *GitRepo) Fetch(auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return err
	}
	return g.repo.Fetch(&git.FetchOptions{Auth: authMethod, Force: true})
}

type SyncDirection int

const (
	SyncNone    SyncDirection = iota
	SyncPush                   // local ahead
	SyncPull                   // remote ahead
	SyncConflict               // both diverged
)

// CompareHeads returns the sync direction after fetching.
// Returns SyncNone if heads are the same, SyncPush if local ahead,
// SyncPull if remote ahead, SyncConflict if both have diverged.
func (g *GitRepo) CompareHeads(branch string) (SyncDirection, *time.Time, *time.Time, error) {
	localRef, err := g.repo.Head()
	if err != nil {
		return SyncNone, nil, nil, fmt.Errorf("local head: %w", err)
	}
	localHash := localRef.Hash()

	remoteRef, err := g.repo.Reference(
		plumbing.NewRemoteReferenceName("origin", branch), true,
	)
	if err != nil {
		// No remote ref yet — local ahead
		if err == plumbing.ErrReferenceNotFound {
			return SyncPush, nil, nil, nil
		}
		return SyncNone, nil, nil, fmt.Errorf("remote ref: %w", err)
	}
	remoteHash := remoteRef.Hash()

	if localHash == remoteHash {
		return SyncNone, nil, nil, nil
	}

	// Check if one is ancestor of the other
	localCommit, err := g.repo.CommitObject(localHash)
	if err != nil {
		return SyncNone, nil, nil, fmt.Errorf("local commit: %w", err)
	}
	remoteCommit, err := g.repo.CommitObject(remoteHash)
	if err != nil {
		return SyncNone, nil, nil, fmt.Errorf("remote commit: %w", err)
	}

	localTime := localCommit.Committer.When
	remoteTime := remoteCommit.Committer.When

	localAncestor, _ := localCommit.IsAncestor(remoteCommit)
	remoteAncestor, _ := remoteCommit.IsAncestor(localCommit)

	if remoteAncestor {
		return SyncPush, &localTime, &remoteTime, nil
	}
	if localAncestor {
		return SyncPull, &localTime, &remoteTime, nil
	}
	return SyncConflict, &localTime, &remoteTime, nil
}

// ForcePush pushes with force, overwriting remote.
func (g *GitRepo) ForcePush(auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return err
	}
	return g.repo.Push(&git.PushOptions{Auth: authMethod, Force: true})
}

// ResetToRemote resets local HEAD to match the remote branch.
func (g *GitRepo) ResetToRemote(branch string) error {
	wt, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	remoteRef, err := g.repo.Reference(
		plumbing.NewRemoteReferenceName("origin", branch), true,
	)
	if err != nil {
		return fmt.Errorf("remote ref: %w", err)
	}
	if err := wt.Reset(&git.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   git.HardReset,
	}); err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	return nil
}

// TestConnection verifies the repo URL is reachable with given credentials.
func TestConnection(repoURL string, auth AuthType, token string) error {
	authMethod, err := buildAuth(auth, token)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	remote := git.NewRemote(nil, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})
	_, err = remote.List(&git.ListOptions{Auth: authMethod})
	if err != nil {
		return fmt.Errorf("remote unreachable: %w", err)
	}
	return nil
}

func buildAuth(auth AuthType, token string) (interface{}, error) {
	switch auth {
	case AuthTypeSSH:
		return buildSSHAuth()
	case AuthTypeToken:
		return &githttp.BasicAuth{
			Username: "token",
			Password: token,
		}, nil
	default:
		return nil, fmt.Errorf("unknown auth type: %s", auth)
	}
}

func buildSSHAuth() (*gitssh.PublicKeys, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}
	sshDir := filepath.Join(home, ".ssh")

	// Try id_ed25519, id_rsa, id_ecdsa in order
	keyNames := []string{"id_ed25519", "id_rsa", "id_ecdsa"}
	for _, name := range keyNames {
		keyPath := filepath.Join(sshDir, name)
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			continue
		}
		signer, err := cryptossh.ParsePrivateKey(keyData)
		if err != nil {
			continue
		}
		return &gitssh.PublicKeys{User: "git", Signer: signer}, nil
	}
	return nil, fmt.Errorf("no SSH private key found in %s", sshDir)
}
```

**Note:** The `initEmpty` function handles first-time sync to an empty remote repo. The error check for "remote repository is empty" needs to use `transport.ErrEmptyRemoteRepository`:

Add to the CloneOrOpen function, import `"github.com/go-git/go-git/v5/plumbing/transport"`, and change the error check:
```go
import (
    // ... other imports
    "errors"
    gittransport "github.com/go-git/go-git/v5/plumbing/transport"
)

// In CloneOrOpen, the error handling for empty remote:
if errors.Is(err, gittransport.ErrEmptyRemoteRepository) {
    return initEmpty(repoPath, repoURL, branch)
}
```

- [ ] **Step 2: Verify build**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/sync/
```
Expected: compiles cleanly (may need minor import fixes)

- [ ] **Step 3: Commit**

```bash
git add backend/sync/git.go
git commit -m "feat(sync): add go-git operations module"
```

---

### Task 6: Create sync service

**Files:**
- Create: `backend/sync/sync_service.go`

- [ ] **Step 1: Write sync_service.go**

```go
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SyncService struct {
	configDir    string
	repoPath     string
	keychain     *Keychain
	configStore  *SyncConfigStore
}

type SyncResult struct {
	Direction SyncDirection
	Message   string
	Conflict  *SyncConflict
}

type SyncConflict struct {
	LocalTime  time.Time
	RemoteTime time.Time
}

func NewSyncService() (*SyncService, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(configDir, "uniTerm")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, err
	}

	return &SyncService{
		configDir:   appDir,
		repoPath:    filepath.Join(appDir, "sync-repo"),
		keychain:    NewKeychain(),
		configStore: NewSyncConfigStore(appDir),
	}, nil
}

// GetConfig returns the current sync configuration.
func (s *SyncService) GetConfig() (SyncConfig, error) {
	return s.configStore.Load()
}

// SaveConfig persists sync configuration and stores the token if provided.
func (s *SyncService) SaveConfig(config SyncConfig, token string) error {
	if config.AuthType == AuthTypeToken && token != "" {
		if err := s.keychain.SetGitToken(token); err != nil {
			return fmt.Errorf("store token: %w", err)
		}
	}
	return s.configStore.Save(config)
}

// getToken returns the stored PAT from keychain, or empty string.
func (s *SyncService) getToken() string {
	token, _ := s.keychain.GetGitToken()
	return token
}

// Sync runs a full sync cycle: encrypt → commit → fetch → compare → push/pull → decrypt.
func (s *SyncService) Sync() (*SyncResult, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if config.RepoURL == "" {
		return nil, fmt.Errorf("sync not configured: repo URL not set")
	}

	encKey, err := s.keychain.GetOrCreateEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("encryption key: %w", err)
	}

	token := s.getToken()

	// 1. Encrypt and stage config files
	if err := EncryptConfigFiles(s.configDir, s.repoPath, encKey); err != nil {
		return nil, fmt.Errorf("encrypt files: %w", err)
	}

	// 2. Open or clone repo
	repo, err := CloneOrOpen(s.repoPath, config.RepoURL, config.Branch, config.AuthType, token)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	// 3. Stage + commit
	committed, err := repo.StageAndCommit("uniTerm config sync")
	if err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// 4. Fetch
	if err := repo.Fetch(config.AuthType, token); err != nil {
		// If fetch fails with "couldn't find remote ref", remote is empty — just push
		if committed {
			if pushErr := repo.Push(config.AuthType, token); pushErr != nil {
				return nil, fmt.Errorf("push to empty remote: %w", pushErr)
			}
			s.updateLastSync()
			return &SyncResult{Direction: SyncPush, Message: "配置已上传"}, nil
		}
		return &SyncResult{Message: "已是最新"}, nil
	}

	// 5. Compare
	direction, localTime, remoteTime, err := repo.CompareHeads(config.Branch)
	if err != nil {
		return nil, fmt.Errorf("compare: %w", err)
	}

	switch direction {
	case SyncNone:
		return &SyncResult{Message: "已是最新"}, nil

	case SyncPush:
		if err := repo.Push(config.AuthType, token); err != nil {
			return nil, fmt.Errorf("push: %w", err)
		}
		s.updateLastSync()
		return &SyncResult{Direction: SyncPush, Message: "配置已上传"}, nil

	case SyncPull:
		if err := repo.Pull(config.AuthType, token); err != nil {
			return nil, fmt.Errorf("pull: %w", err)
		}
		// Decrypt pulled files back to config dir
		if err := DecryptConfigFiles(s.repoPath, s.configDir, encKey); err != nil {
			return nil, fmt.Errorf("decrypt files: %w", err)
		}
		s.updateLastSync()
		return &SyncResult{Direction: SyncPull, Message: "配置已下载"}, nil

	case SyncConflict:
		if localTime == nil || remoteTime == nil {
			localTime = &time.Time{}
			remoteTime = &time.Time{}
		}
		return &SyncResult{
			Direction: SyncConflict,
			Conflict: &SyncConflict{
				LocalTime:  *localTime,
				RemoteTime: *remoteTime,
			},
		}, nil
	}

	return &SyncResult{Message: "已是最新"}, nil
}

// ResolveConflict handles a conflict by forcing push or reset.
// useLocal=true: force push local; useLocal=false: reset to remote.
func (s *SyncService) ResolveConflict(useLocal bool) (*SyncResult, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	token := s.getToken()

	repo, err := CloneOrOpen(s.repoPath, config.RepoURL, config.Branch, config.AuthType, token)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	if useLocal {
		if err := repo.ForcePush(config.AuthType, token); err != nil {
			return nil, fmt.Errorf("force push: %w", err)
		}
		s.updateLastSync()
		return &SyncResult{Direction: SyncPush, Message: "已用本地配置覆盖远端"}, nil
	}

	if err := repo.ResetToRemote(config.Branch); err != nil {
		return nil, fmt.Errorf("reset to remote: %w", err)
	}

	encKey, err := s.keychain.GetOrCreateEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("encryption key: %w", err)
	}
	if err := DecryptConfigFiles(s.repoPath, s.configDir, encKey); err != nil {
		return nil, fmt.Errorf("decrypt files: %w", err)
	}

	s.updateLastSync()
	return &SyncResult{Direction: SyncPull, Message: "已用远端配置覆盖本地"}, nil
}

// TestConnection verifies the repo is reachable with the stored credentials.
func (s *SyncService) TestConnection() error {
	config, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if config.RepoURL == "" {
		return fmt.Errorf("仓库地址未设置")
	}
	token := s.getToken()
	return TestConnection(config.RepoURL, config.AuthType, token)
}

func (s *SyncService) updateLastSync() {
	config, _ := s.configStore.Load()
	config.LastSyncAt = time.Now()
	_ = s.configStore.Save(config)
}

// GetLastSyncTime returns the formatted last sync time string.
func (s *SyncService) GetLastSyncTime() string {
	config, _ := s.configStore.Load()
	if config.LastSyncAt.IsZero() {
		return "从未同步"
	}
	return config.LastSyncAt.Format("2006-01-02 15:04:05")
}

// IsAutoSyncEnabled returns whether auto sync is enabled and configured.
func (s *SyncService) IsAutoSyncEnabled() bool {
	config, _ := s.configStore.Load()
	return config.AutoSync && config.RepoURL != ""
}

// RepoPath returns the local git repo path (for testing/inspection).
func (s *SyncService) RepoPath() string {
	return s.repoPath
}

// Keychain returns the keychain instance (for password migration use).
func (s *SyncService) Keychain() *Keychain {
	return s.keychain
}
```

- [ ] **Step 2: Verify build**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/sync/
```
Expected: compiles cleanly

- [ ] **Step 3: Commit**

```bash
git add backend/sync/sync_service.go
git commit -m "feat(sync): add sync orchestration service"
```

---

### Task 7: Modify ConnectionStore for keychain password migration

**Files:**
- Modify: `backend/store/connection_store.go`

- [ ] **Step 1: Refactor ConnectionStore to accept keychain dependency**

Change `ConnectionStore` to accept an optional `KeychainProvider` interface:

In `backend/sync/keychain.go`, define and export an interface:

```go
// PasswordStore is the interface for reading/writing connection passwords.
// This avoids circular imports between store and sync packages.
```

Wait — this would create a dependency from `store` → `sync` or require an interface in `store`. Given the existing patterns, the cleanest approach is to define a minimal interface in the `store` package and have `sync.Keychain` satisfy it, OR to have the migration happen externally via `App.startup()`.

**Better approach:** Keep migration out of `ConnectionStore` itself. Instead, add a migration step in `app.go:startup()` that:
1. Loads connections
2. Iterates and migrates passwords to keychain
3. Re-saves cleaned connections

Then modify `ConnectionStore.Save()` to extract passwords to the external keychain before writing, and `Load()` to inject passwords from keychain after reading.

We'll use a simple callback/interface approach. Define in `backend/store/`:

```go
// In a new file or in connection_store.go:
type PasswordStore interface {
    GetPassword(connID string) (string, error)
    SetPassword(connID, password string) error
    DeletePassword(connID string) error
}
```

Then modify `ConnectionStore` struct to hold an optional `PasswordStore`:

**Modified `connection_store.go`:**

```go
package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ys-ll/uniterm/backend/session"
)

const storeFileName = "connections.json"

type PasswordStore interface {
	GetPassword(connID string) (string, error)
	SetPassword(connID, password string) error
	DeletePassword(connID string) error
}

type ConnectionStore struct {
	configDir     string
	passwordStore PasswordStore // nil = password stored in JSON (backward compat)
}

func NewConnectionStore() (*ConnectionStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(configDir, "uniTerm")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, err
	}
	return &ConnectionStore{configDir: appDir}, nil
}

// SetPasswordStore sets the external password store. Once set, passwords
// are written to the store and cleared from the JSON file.
func (s *ConnectionStore) SetPasswordStore(ps PasswordStore) {
	s.passwordStore = ps
}

func (s *ConnectionStore) filePath() string {
	return filepath.Join(s.configDir, storeFileName)
}

func (s *ConnectionStore) Save(data session.ConnectionStoreData) error {
	// Extract passwords to external store
	for i := range data.Connections {
		conn := &data.Connections[i]
		if conn.AuthType != "password" || conn.Password == "" {
			continue
		}
		if s.passwordStore != nil {
			_ = s.passwordStore.SetPassword(conn.ID, conn.Password)
		}
		conn.Password = ""
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(), jsonData, 0600)
}

func (s *ConnectionStore) Load() (session.ConnectionStoreData, error) {
	fileData, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return session.ConnectionStoreData{
				Groups:      []session.ConnectionGroup{},
				Connections: []session.ConnectionConfig{},
			}, nil
		}
		return session.ConnectionStoreData{}, err
	}

	var data session.ConnectionStoreData
	if err := json.Unmarshal(fileData, &data); err == nil && (data.Groups != nil || data.Connections != nil) {
		if data.Groups == nil {
			data.Groups = []session.ConnectionGroup{}
		}
		if data.Connections == nil {
			data.Connections = []session.ConnectionConfig{}
		}
		s.populatePasswords(&data)
		return data, nil
	}

	var connections []session.ConnectionConfig
	if err := json.Unmarshal(fileData, &connections); err != nil {
		return session.ConnectionStoreData{}, err
	}
	data = session.ConnectionStoreData{
		Groups:      []session.ConnectionGroup{},
		Connections: connections,
	}
	s.populatePasswords(&data)
	return data, nil
}

func (s *ConnectionStore) populatePasswords(data *session.ConnectionStoreData) {
	needsSave := false
	for i := range data.Connections {
		conn := &data.Connections[i]
		if conn.AuthType != "password" {
			continue
		}

		if s.passwordStore != nil {
			// Migration: if JSON still has plaintext password, move to keychain
			if conn.Password != "" {
				_ = s.passwordStore.SetPassword(conn.ID, conn.Password)
				conn.Password = ""
				needsSave = true
			}

			// Load from keychain
			if pw, err := s.passwordStore.GetPassword(conn.ID); err == nil && pw != "" {
				conn.Password = pw
			}
		}
	}

	if needsSave {
		// Save cleaned JSON (passwords migrated out)
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return
		}
		_ = os.WriteFile(s.filePath(), jsonData, 0600)
	}
}
```

- [ ] **Step 2: Verify build compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./...
```
Expected: clean exit

- [ ] **Step 3: Commit**

```bash
git add backend/store/connection_store.go backend/sync/
git commit -m "feat(sync): add keychain password migration to ConnectionStore"
```

---

### Task 8: Wire sync service into App and expose Wails methods

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Add sync fields to App struct**

Add at line 32 (after `settingsStore`):
```go
syncService *sync.SyncService
```

Add import `"github.com/ys-ll/uniterm/backend/sync"` to imports.

- [ ] **Step 2: Initialize sync service in startup()**

Add after line 82 (`a.settingsStore = ss`):
```go
	syncSvc, err := sync.NewSyncService()
	if err != nil {
		log.Writef("Failed to create sync service: %v", err)
	} else {
		a.syncService = syncSvc
		// Wire keychain into connection store for password migration
		if a.connectionStore != nil {
			a.connectionStore.SetPasswordStore(syncSvc.Keychain())
		}
		// Auto-sync on startup if enabled
		if syncSvc.IsAutoSyncEnabled() {
			go func() {
				result, err := syncSvc.Sync()
				if err != nil {
					log.Writef("Auto-sync on startup failed: %v", err)
				} else if result.Direction == sync.SyncConflict {
					runtime.EventsEmit(a.ctx, "sync:conflict", map[string]interface{}{
						"localTime":  result.Conflict.LocalTime.Format(time.RFC3339),
						"remoteTime": result.Conflict.RemoteTime.Format(time.RFC3339),
					})
				}
			}()
		}
	}
```

Add `"time"` to imports.

- [ ] **Step 3: Add Wails bound methods on App**

Add after the existing store methods (around line 158):

```go
// SyncGetConfig returns the sync configuration.
func (a *App) SyncGetConfig() (sync.SyncConfig, error) {
	if a.syncService == nil {
		return sync.SyncConfig{}, fmt.Errorf("sync service not initialized")
	}
	return a.syncService.GetConfig()
}

// SyncSaveConfig saves the sync configuration.
func (a *App) SyncSaveConfig(config sync.SyncConfig, token string) error {
	if a.syncService == nil {
		return fmt.Errorf("sync service not initialized")
	}
	return a.syncService.SaveConfig(config, token)
}

// SyncNow runs an immediate sync.
func (a *App) SyncNow() (*sync.SyncResult, error) {
	if a.syncService == nil {
		return nil, fmt.Errorf("sync service not initialized")
	}
	result, err := a.syncService.Sync()
	if err != nil {
		return nil, err
	}
	if result.Direction == sync.SyncConflict {
		runtime.EventsEmit(a.ctx, "sync:conflict", map[string]interface{}{
			"localTime":  result.Conflict.LocalTime.Format(time.RFC3339),
			"remoteTime": result.Conflict.RemoteTime.Format(time.RFC3339),
		})
	}
	return result, nil
}

// SyncResolveConflict resolves a sync conflict.
func (a *App) SyncResolveConflict(useLocal bool) (*sync.SyncResult, error) {
	if a.syncService == nil {
		return nil, fmt.Errorf("sync service not initialized")
	}
	result, err := a.syncService.ResolveConflict(useLocal)
	if err != nil {
		return nil, err
	}
	// If we pulled remote config, notify stores to reload
	if result.Direction == sync.SyncPull {
		if data, err := a.connectionStore.Load(); err == nil {
			runtime.EventsEmit(a.ctx, "store:connections:changed", data)
		}
	}
	return result, nil
}

// SyncTestConnection tests the repository connection.
func (a *App) SyncTestConnection() error {
	if a.syncService == nil {
		return fmt.Errorf("sync service not initialized")
	}
	return a.syncService.TestConnection()
}

// SyncGetLastSyncTime returns the last sync time string.
func (a *App) SyncGetLastSyncTime() string {
	if a.syncService == nil {
		return "从未同步"
	}
	return a.syncService.GetLastSyncTime()
}
```

- [ ] **Step 4: Add auto-sync trigger on config save**

Modify `SaveConnections` and `SaveAIConfig` to trigger auto-sync:

```go
func (a *App) SaveConnections(data session.ConnectionStoreData) error {
	if a.connectionStore == nil {
		return fmt.Errorf("connection store not initialized")
	}
	err := a.connectionStore.Save(data)
	if err == nil {
		runtime.EventsEmit(a.ctx, "store:connections:changed", data)
		// Auto-sync if enabled
		a.triggerAutoSync()
	}
	return err
}

func (a *App) SaveAIConfig(config store.AIConfig) error {
	if a.aiConfigStore == nil {
		return fmt.Errorf("AI config store not initialized")
	}
	err := a.aiConfigStore.Save(config)
	if err == nil {
		a.triggerAutoSync()
	}
	return err
}

func (a *App) triggerAutoSync() {
	if a.syncService == nil || !a.syncService.IsAutoSyncEnabled() {
		return
	}
	go func() {
		result, err := a.syncService.Sync()
		if err != nil {
			log.Writef("Auto-sync failed: %v", err)
		} else if result.Direction == sync.SyncConflict {
			runtime.EventsEmit(a.ctx, "sync:conflict", map[string]interface{}{
				"localTime":  result.Conflict.LocalTime.Format(time.RFC3339),
				"remoteTime": result.Conflict.RemoteTime.Format(time.RFC3339),
			})
		}
	}()
}
```

- [ ] **Step 5: Verify build compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./...
```
Expected: clean exit, no errors

- [ ] **Step 6: Commit**

```bash
git add app.go
git commit -m "feat(sync): wire sync service into App, expose Wails methods"
```

---

### Task 9: Regenerate Wails TypeScript bindings

**Files:** (auto-generated, will change)
- `frontend/wailsjs/go/main/App.d.ts`
- `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Run wails generate**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm
wails generate module
```

- [ ] **Step 2: Verify new methods appear in App.d.ts**

Check that these new functions are declared in `frontend/wailsjs/go/main/App.d.ts`:
- `SyncGetConfig`
- `SyncSaveConfig`
- `SyncNow`
- `SyncResolveConflict`
- `SyncTestConnection`
- `SyncGetLastSyncTime`

```bash
grep "Sync" frontend/wailsjs/go/main/App.d.ts
```

- [ ] **Step 3: Commit**

```bash
git add frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts
git commit -m "chore: regenerate Wails bindings for sync methods"
```

---

### Task 10: Create frontend sync Pinia store

**Files:**
- Create: `frontend/src/stores/syncStore.ts`

- [ ] **Step 1: Write syncStore.ts**

```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  SyncGetConfig,
  SyncSaveConfig,
  SyncNow,
  SyncResolveConflict,
  SyncTestConnection,
  SyncGetLastSyncTime,
} from '../../wailsjs/go/main/App'
import { sync } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime'

export interface SyncConfig {
  repoUrl: string
  branch: string
  authType: 'ssh' | 'token'
  autoSync: boolean
  lastSyncAt: string
}

export interface SyncResult {
  direction: number // 0=none, 1=push, 2=pull, 3=conflict
  message: string
  conflict?: SyncConflict
}

export interface SyncConflict {
  localTime: string
  remoteTime: string
}

export const useSyncStore = defineStore('sync', () => {
  const config = ref<SyncConfig>({
    repoUrl: '',
    branch: 'main',
    authType: 'ssh',
    autoSync: false,
    lastSyncAt: '',
  })
  const lastSyncTime = ref('从未同步')
  const syncing = ref(false)
  const testingConnection = ref(false)
  const conflict = ref<SyncConflict | null>(null)
  const lastResult = ref('')

  async function loadConfig() {
    try {
      const cfg = await SyncGetConfig()
      config.value = {
        repoUrl: cfg.repoUrl || '',
        branch: cfg.branch || 'main',
        authType: (cfg.authType as 'ssh' | 'token') || 'ssh',
        autoSync: cfg.autoSync || false,
        lastSyncAt: cfg.lastSyncAt || '',
      }
    } catch (e) {
      console.error('Load sync config failed:', e)
    }
    try {
      lastSyncTime.value = await SyncGetLastSyncTime()
    } catch (_) {}
  }

  async function saveConfig(token: string = '') {
    try {
      const cfg = new sync.SyncConfig()
      cfg.repoUrl = config.value.repoUrl
      cfg.branch = config.value.branch
      cfg.authType = config.value.authType
      cfg.autoSync = config.value.autoSync
      cfg.lastSyncAt = config.value.lastSyncAt
      await SyncSaveConfig(cfg, token)
    } catch (e) {
      console.error('Save sync config failed:', e)
      throw e
    }
  }

  async function doSync(): Promise<SyncResult | null> {
    syncing.value = true
    try {
      const result = await SyncNow()
      if (result.direction === 3) {
        // Conflict
        conflict.value = result.conflict || null
      } else {
        conflict.value = null
      }
      lastResult.value = result.message || ''
      await updateLastSyncTime()
      return {
        direction: result.direction,
        message: result.message || '',
        conflict: result.conflict
          ? { localTime: result.conflict.localTime, remoteTime: result.conflict.remoteTime }
          : undefined,
      }
    } catch (e: any) {
      lastResult.value = e?.message || String(e)
      return null
    } finally {
      syncing.value = false
    }
  }

  async function resolveConflict(useLocal: boolean): Promise<SyncResult | null> {
    syncing.value = true
    try {
      const result = await SyncResolveConflict(useLocal)
      conflict.value = null
      lastResult.value = result.message || ''
      await updateLastSyncTime()
      return {
        direction: result.direction,
        message: result.message || '',
      }
    } catch (e: any) {
      lastResult.value = e?.message || String(e)
      return null
    } finally {
      syncing.value = false
    }
  }

  async function testConnection(): Promise<string | null> {
    testingConnection.value = true
    try {
      await SyncTestConnection()
      return null // success
    } catch (e: any) {
      return e?.message || String(e)
    } finally {
      testingConnection.value = false
    }
  }

  async function updateLastSyncTime() {
    try {
      lastSyncTime.value = await SyncGetLastSyncTime()
    } catch (_) {}
  }

  // Listen for conflict events from auto-sync
  EventsOn('sync:conflict', (data: SyncConflict) => {
    conflict.value = data
  })

  return {
    config,
    lastSyncTime,
    syncing,
    testingConnection,
    conflict,
    lastResult,
    loadConfig,
    saveConfig,
    doSync,
    resolveConflict,
    testConnection,
  }
})
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vue-tsc --noEmit
```
Expected: may have model import issues depending on wails generate output; fix type references as needed.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/stores/syncStore.ts
git commit -m "feat(sync): add frontend sync Pinia store"
```

---

### Task 11: Add sync tab to SettingsTab.vue

**Files:**
- Modify: `frontend/src/components/SettingsTab.vue`

- [ ] **Step 1: Add sync category to the `categories` computed array**

At line ~252, add after the `ai` entry:
```typescript
{ key: 'sync', label: t('settings.sync'), icon: RefreshCw },
```

Import the `RefreshCw` icon from `lucide-vue-next` at top:
```typescript
import { Settings, Monitor, MessageCircleMore, Info, RefreshCw } from 'lucide-vue-next'
```

- [ ] **Step 2: Add the sync tab content template**

Add after the existing AI section `v-if` block (after line ~203), before the About section:

```vue
      <!-- Sync settings -->
      <div v-if="activeCategory === 'sync'" class="settings-section sync-settings">
        <div class="section-header">
          <h2>{{ t('settings.sync') }}</h2>
          <p class="section-desc">{{ t('settings.syncDesc') }}</p>
        </div>

        <div class="sync-warning">
          <AlertTriangle :size="16" />
          <span>{{ t('settings.syncWarning') }}</span>
        </div>

        <div class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncRepoUrl') }}</label>
            <p class="setting-desc">{{ t('settings.syncRepoUrlDesc') }}</p>
          </div>
          <div class="setting-control">
            <el-input
              v-model="syncStore.config.repoUrl"
              :placeholder="t('settings.syncRepoUrlPlaceholder')"
              size="default"
              style="width: 400px"
            />
          </div>
        </div>

        <div class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncAuthType') }}</label>
          </div>
          <div class="setting-control">
            <el-radio-group v-model="syncStore.config.authType">
              <el-radio value="ssh">SSH Key</el-radio>
              <el-radio value="token">Personal Access Token</el-radio>
            </el-radio-group>
          </div>
        </div>

        <div v-if="syncStore.config.authType === 'token'" class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncToken') }}</label>
          </div>
          <div class="setting-control">
            <el-input
              v-model="tokenInput"
              :type="showToken ? 'text' : 'password'"
              :placeholder="t('settings.syncTokenPlaceholder')"
              size="default"
              style="width: 300px"
            >
              <template #suffix>
                <el-button link @click="showToken = !showToken">
                  {{ showToken ? t('settings.syncHide') : t('settings.syncShow') }}
                </el-button>
              </template>
            </el-input>
          </div>
        </div>

        <div class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncAuto') }}</label>
            <p class="setting-desc">{{ t('settings.syncAutoDesc') }}</p>
          </div>
          <div class="setting-control">
            <el-switch v-model="syncStore.config.autoSync" />
          </div>
        </div>

        <div class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncLastTime') }}</label>
          </div>
          <div class="setting-control sync-time">
            {{ syncStore.lastSyncTime }}
          </div>
        </div>

        <div class="sync-actions">
          <el-button
            :loading="syncStore.testingConnection"
            @click="handleTestConnection"
          >
            {{ t('settings.syncTestConnection') }}
          </el-button>
          <el-button
            type="primary"
            :loading="syncStore.syncing"
            @click="handleSyncNow"
          >
            {{ t('settings.syncNow') }}
          </el-button>
        </div>

        <div v-if="syncStore.lastResult" class="sync-result">
          {{ syncStore.lastResult }}
        </div>
      </div>
```

- [ ] **Step 3: Add script setup imports and logic**

Add import at top:
```typescript
import { useSyncStore } from '../stores/syncStore'
import { AlertTriangle } from 'lucide-vue-next'
```

Add reactive state and functions inside `<script setup>`:
```typescript
const syncStore = useSyncStore()
const tokenInput = ref('')
const showToken = ref(false)

async function handleTestConnection() {
  await syncStore.saveConfig(tokenInput.value)
  const err = await syncStore.testConnection()
  if (err) {
    ElMessage.error(t('settings.syncTestFailed', { error: err }))
  } else {
    ElMessage.success(t('settings.syncTestSuccess'))
  }
}

async function handleSyncNow() {
  await syncStore.saveConfig(tokenInput.value)
  const result = await syncStore.doSync()
  if (!result) {
    ElMessage.error(syncStore.lastResult || t('settings.syncFailed'))
    return
  }
  if (result.direction === 3) {
    // conflict — handled by the conflict dialog in Task 12
    return
  }
  ElMessage.success(result.message || t('settings.syncSuccess'))
}

// Load config on mount
syncStore.loadConfig()
```

Add `ref` and `ElMessage` imports if not already present:
```typescript
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
```

- [ ] **Step 4: Add i18n keys for sync section**

In `frontend/src/i18n/index.ts`, add to both `'zh-CN'` and `'en'` blocks:

zh-CN:
```typescript
'settings.sync': '配置同步',
'settings.syncDesc': '将连接配置和 AI 配置同步到 Git 私有仓库',
'settings.syncWarning': '请使用私有仓库。配置文件包含加密后的敏感信息，请勿使用公开仓库进行同步。',
'settings.syncRepoUrl': 'Git 仓库',
'settings.syncRepoUrlDesc': '支持 GitHub、Gitee 等 Git 服务的 HTTPS 或 SSH 地址',
'settings.syncRepoUrlPlaceholder': 'https://github.com/user/config.git',
'settings.syncAuthType': '认证方式',
'settings.syncToken': 'Token',
'settings.syncTokenPlaceholder': '输入 Personal Access Token',
'settings.syncShow': '显示',
'settings.syncHide': '隐藏',
'settings.syncAuto': '自动同步',
'settings.syncAutoDesc': '开启后保存配置时自动推送，启动时自动拉取',
'settings.syncLastTime': '上次同步',
'settings.syncTestConnection': '测试连接',
'settings.syncNow': '立即同步',
'settings.syncTestSuccess': '连接测试成功',
'settings.syncTestFailed': '连接测试失败: {error}',
'settings.syncSuccess': '同步完成',
'settings.syncFailed': '同步失败',
```

en:
```typescript
'settings.sync': 'Config Sync',
'settings.syncDesc': 'Sync connection and AI configs to a private Git repository',
'settings.syncWarning': 'Please use a private repository. Config files contain encrypted sensitive data. Do not use a public repository.',
'settings.syncRepoUrl': 'Git Repository',
'settings.syncRepoUrlDesc': 'HTTPS or SSH URL for GitHub, Gitee, or other Git services',
'settings.syncRepoUrlPlaceholder': 'https://github.com/user/config.git',
'settings.syncAuthType': 'Authentication',
'settings.syncToken': 'Token',
'settings.syncTokenPlaceholder': 'Enter Personal Access Token',
'settings.syncShow': 'Show',
'settings.syncHide': 'Hide',
'settings.syncAuto': 'Auto Sync',
'settings.syncAutoDesc': 'Automatically push on save and pull on startup',
'settings.syncLastTime': 'Last Sync',
'settings.syncTestConnection': 'Test Connection',
'settings.syncNow': 'Sync Now',
'settings.syncTestSuccess': 'Connection test succeeded',
'settings.syncTestFailed': 'Connection test failed: {error}',
'settings.syncSuccess': 'Sync completed',
'settings.syncFailed': 'Sync failed',
```

- [ ] **Step 5: Add sync styles**

In the `<style scoped>` section of `SettingsTab.vue`, add:
```css
.sync-warning {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: var(--el-color-warning-light-9);
  border: 1px solid var(--el-color-warning-light-5);
  border-radius: 6px;
  margin-bottom: 20px;
  color: var(--el-color-warning-dark-2);
  font-size: 13px;
}

.sync-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}

.sync-result {
  margin-top: 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.sync-time {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
```

- [ ] **Step 6: Verify TypeScript compilation**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vue-tsc --noEmit
```
Expected: clean exit (fix type errors as needed)

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/SettingsTab.vue frontend/src/i18n/index.ts
git commit -m "feat(sync): add sync settings tab with i18n"
```

---

### Task 12: Create conflict dialog component

**Files:**
- Create: `frontend/src/components/SyncConflictDialog.vue`
- Modify: `frontend/src/App.vue` (add dialog to app root)

- [ ] **Step 1: Write SyncConflictDialog.vue**

```vue
<template>
  <el-dialog
    v-model="visible"
    :title="t('sync.conflictTitle')"
    width="480px"
    :close-on-click-modal="false"
    @close="handleCancel"
  >
    <div class="conflict-body">
      <p>{{ t('sync.conflictDesc') }}</p>
      <div class="conflict-times">
        <div class="conflict-time">
          <span class="time-label">{{ t('sync.conflictLocal') }}：</span>
          <span>{{ formatTime(syncStore.conflict?.localTime) }}</span>
        </div>
        <div class="conflict-time">
          <span class="time-label">{{ t('sync.conflictRemote') }}：</span>
          <span>{{ formatTime(syncStore.conflict?.remoteTime) }}</span>
        </div>
      </div>

      <el-radio-group v-model="choice" class="conflict-choice">
        <el-radio value="local">{{ t('sync.conflictUseLocal') }}</el-radio>
        <el-radio value="remote">{{ t('sync.conflictUseRemote') }}</el-radio>
      </el-radio-group>
    </div>

    <template #footer>
      <el-button @click="handleCancel">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="syncing" @click="handleConfirm">
        {{ t('common.confirm') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from '../i18n'
import { useSyncStore } from '../stores/syncStore'
import { ElMessage } from 'element-plus'

const { t } = useI18n()
const syncStore = useSyncStore()
const choice = ref<'local' | 'remote'>('local')
const syncing = ref(false)

const visible = computed({
  get: () => syncStore.conflict !== null,
  set: (v) => { if (!v) syncStore.conflict = null },
})

function formatTime(timeStr?: string): string {
  if (!timeStr) return '-'
  try {
    const d = new Date(timeStr)
    return d.toLocaleString()
  } catch {
    return timeStr
  }
}

async function handleConfirm() {
  syncing.value = true
  try {
    const result = await syncStore.resolveConflict(choice.value === 'local')
    if (result) {
      ElMessage.success(result.message || t('settings.syncSuccess'))
    } else {
      ElMessage.error(syncStore.lastResult || t('settings.syncFailed'))
    }
  } finally {
    syncing.value = false
  }
}

function handleCancel() {
  syncStore.conflict = null
}
</script>

<style scoped>
.conflict-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.conflict-times {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  font-size: 13px;
}

.time-label {
  font-weight: 500;
}

.conflict-choice {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.conflict-choice .el-radio {
  margin-right: 0;
}
</style>
```

- [ ] **Step 2: Add i18n keys for conflict dialog**

In `frontend/src/i18n/index.ts`, add to both locales:

zh-CN:
```typescript
'sync.conflictTitle': '配置冲突',
'sync.conflictDesc': '本地和远端都有未同步的修改，请选择覆盖方向：',
'sync.conflictLocal': '本地修改时间',
'sync.conflictRemote': '远端修改时间',
'sync.conflictUseLocal': '用本地覆盖远端',
'sync.conflictUseRemote': '用远端覆盖本地',
```

en:
```typescript
'sync.conflictTitle': 'Config Conflict',
'sync.conflictDesc': 'Both local and remote have unsynchronized changes. Choose which to keep:',
'sync.conflictLocal': 'Local modification time',
'sync.conflictRemote': 'Remote modification time',
'sync.conflictUseLocal': 'Use local, overwrite remote',
'sync.conflictUseRemote': 'Use remote, overwrite local',
```

- [ ] **Step 3: Add SyncConflictDialog to App.vue**

In `frontend/src/App.vue`, add import and component:
```vue
<script setup lang="ts">
import SyncConflictDialog from './components/SyncConflictDialog.vue'
</script>

<template>
  <!-- Add at end of template, before closing root element -->
  <SyncConflictDialog />
</template>
```

- [ ] **Step 4: Verify TypeScript compilation**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vue-tsc --noEmit
```
Expected: clean exit

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/SyncConflictDialog.vue frontend/src/App.vue frontend/src/i18n/index.ts
git commit -m "feat(sync): add conflict resolution dialog"
```

---

### Task 13: End-to-end build verification

**Files:** (none created/modified, verification only)

- [ ] **Step 1: Full Go build**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build -o /dev/null ./...
```
Expected: clean exit

- [ ] **Step 2: Full frontend build**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npx vite build
```
Expected: clean exit, no warnings

- [ ] **Step 3: Wails dev build (optional, requires Windows)**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && wails build
```
Expected: produces binary

- [ ] **Step 4: Commit any final fixes**

```bash
git add -A
git status  # review what changed
git commit -m "chore(sync): final build fixes"
```
