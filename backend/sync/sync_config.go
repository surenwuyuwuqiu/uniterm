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
