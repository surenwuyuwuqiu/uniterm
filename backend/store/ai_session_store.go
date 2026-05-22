package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const aiSessionFileName = "ai-sessions.json"

type AISessionData struct {
	Sessions        []AISessionEntry `json:"sessions"`
	CurrentSessionID string          `json:"currentSessionId"`
}

type AISessionEntry struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	CreatedAt int64               `json:"createdAt"`
	UpdatedAt int64               `json:"updatedAt"`
	Messages  []AIMessageEntry    `json:"messages"`
}

type AIMessageEntry struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	Content     string `json:"content"`
	ToolCallID  string `json:"tool_call_id,omitempty"`
	RawAPIMsg   string `json:"_rawApiMsg,omitempty"`
}

type AISessionStore struct {
	configDir string
}

func NewAISessionStore() (*AISessionStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(configDir, "uniTerm")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, err
	}
	return &AISessionStore{configDir: appDir}, nil
}

func (s *AISessionStore) filePath() string {
	return filepath.Join(s.configDir, aiSessionFileName)
}

func (s *AISessionStore) Save(data AISessionData) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(), jsonData, 0600)
}

func (s *AISessionStore) Load() (AISessionData, error) {
	fileData, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return AISessionData{Sessions: []AISessionEntry{}}, nil
		}
		return AISessionData{}, err
	}
	var data AISessionData
	if err := json.Unmarshal(fileData, &data); err != nil {
		return AISessionData{Sessions: []AISessionEntry{}}, nil
	}
	return data, nil
}
