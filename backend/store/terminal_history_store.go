package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const terminalHistoryFileName = "terminal-history.json"
const maxHistorySize = 500

type TerminalHistoryStore struct {
	configDir string
}

type HistoryEntry struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

type TerminalHistoryData struct {
	Entries []HistoryEntry `json:"entries"`
}

func NewTerminalHistoryStore(configDir string) *TerminalHistoryStore {
	return &TerminalHistoryStore{configDir: configDir}
}

func (s *TerminalHistoryStore) filePath() string {
	return filepath.Join(s.configDir, terminalHistoryFileName)
}

func (s *TerminalHistoryStore) Save(entries []HistoryEntry) error {
	// Deduplicate by Command: keep last occurrence
	seen := make(map[string]bool)
	result := make([]HistoryEntry, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.Command == "" || seen[entry.Command] {
			continue
		}
		seen[entry.Command] = true
		result = append([]HistoryEntry{entry}, result...)
	}
	// Trim to max
	if len(result) > maxHistorySize {
		result = result[len(result)-maxHistorySize:]
	}
	data := TerminalHistoryData{Entries: result}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(), jsonData, 0600)
}

func (s *TerminalHistoryStore) Load() ([]HistoryEntry, error) {
	fileData, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryEntry{}, nil
		}
		return nil, err
	}
	var data TerminalHistoryData
	if err := json.Unmarshal(fileData, &data); err != nil {
		// Old format or corrupt: clear file
		_ = os.Remove(s.filePath())
		return []HistoryEntry{}, nil
	}
	// Defensive: if unmarshaled but Entries is nil/empty and file had content,
	// treat as old format (old format had "commands" not "entries")
	if len(data.Entries) == 0 && len(fileData) > 10 {
		var oldFormat struct {
			Commands []string `json:"commands"`
		}
		if err := json.Unmarshal(fileData, &oldFormat); err == nil && len(oldFormat.Commands) > 0 {
			_ = os.Remove(s.filePath())
			return []HistoryEntry{}, nil
		}
	}
	return data.Entries, nil
}

func (s *TerminalHistoryStore) DeleteByIDs(ids []string) error {
	entries, err := s.Load()
	if err != nil {
		return err
	}
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	filtered := make([]HistoryEntry, 0, len(entries))
	for _, entry := range entries {
		if !idSet[entry.ID] {
			filtered = append(filtered, entry)
		}
	}
	return s.Save(filtered)
}
