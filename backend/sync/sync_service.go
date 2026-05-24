package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SyncService struct {
	configDir   string
	repoPath    string
	keychain    *Keychain
	configStore *SyncConfigStore
}

type SyncResult struct {
	Direction SyncDirection
	Message   string
	Conflict  *ConflictInfo
}

type ConflictInfo struct {
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
// If the repo URL changed, the old local sync repo is removed so CloneOrOpen
// will clone from the new URL on the next sync.
func (s *SyncService) SaveConfig(config SyncConfig, token string) error {
	if config.AuthType == AuthTypeToken && token != "" {
		if err := s.keychain.SetGitToken(token); err != nil {
			return fmt.Errorf("store token: %w", err)
		}
	}

	oldConfig, _ := s.configStore.Load()
	if oldConfig.RepoURL != "" && oldConfig.RepoURL != config.RepoURL {
		// URL changed — purge old clone so we re-clone from new remote
		if err := os.RemoveAll(s.repoPath); err != nil {
			return fmt.Errorf("remove old sync repo: %w", err)
		}
	}

	return s.configStore.Save(config)
}

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
		// If fetch fails, remote might be empty — just push if we have local changes
		if committed {
			if pushErr := repo.Push(config.AuthType, token); pushErr != nil {
				return nil, fmt.Errorf("push to empty remote: %w", pushErr)
			}
			s.updateLastSync()
			return &SyncResult{Direction: SyncPush, Message: "配置已上传"}, nil
		}
		return &SyncResult{Message: "已是最新"}, nil
	}

	// 5. Compare heads
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
		if err := DecryptConfigFiles(s.repoPath, s.configDir, encKey); err != nil {
			return nil, fmt.Errorf("decrypt files: %w", err)
		}
		s.updateLastSync()
		return &SyncResult{Direction: SyncPull, Message: "配置已下载"}, nil

	case SyncConflict:
		if localTime == nil {
			t := time.Time{}
			localTime = &t
		}
		if remoteTime == nil {
			t := time.Time{}
			remoteTime = &t
		}
		return &SyncResult{
			Direction: SyncConflict,
			Conflict: &ConflictInfo{
				LocalTime:  *localTime,
				RemoteTime: *remoteTime,
			},
		}, nil
	}

	return &SyncResult{Message: "已是最新"}, nil
}

// ResolveConflict handles a conflict by forcing push or reset.
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

// TestConnection verifies the repo is reachable with stored credentials.
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

// RepoPath returns the local git repo path.
func (s *SyncService) RepoPath() string {
	return s.repoPath
}

// Keychain returns the keychain instance.
func (s *SyncService) Keychain() *Keychain {
	return s.keychain
}
