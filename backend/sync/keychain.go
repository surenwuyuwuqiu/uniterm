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
