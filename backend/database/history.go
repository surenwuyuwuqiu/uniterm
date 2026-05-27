package database

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "time"

    "github.com/google/uuid"
)

type HistoryEntry struct {
    ID         string    `json:"id"`
    SQL        string    `json:"sql"`
    ExecutedAt time.Time `json:"executedAt"`
    Duration   int64     `json:"durationMs"`
    Error      string    `json:"error,omitempty"`
    RowCount   int       `json:"rowCount,omitempty"`
}

const maxHistoryEntries = 500

func historyDir() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    dir := filepath.Join(home, ".uniterm", "db_history")
    if err := os.MkdirAll(dir, 0700); err != nil {
        return "", err
    }
    return dir, nil
}

func historyPath(connID string) (string, error) {
    dir, err := historyDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(dir, fmt.Sprintf("%s.json", connID)), nil
}

func LoadHistory(connID string) ([]HistoryEntry, error) {
    path, err := historyPath(connID)
    if err != nil {
        return nil, err
    }

    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return []HistoryEntry{}, nil
        }
        return nil, err
    }

    var entries []HistoryEntry
    if err := json.Unmarshal(data, &entries); err != nil {
        return nil, err
    }
    if entries == nil {
        entries = []HistoryEntry{}
    }
    return entries, nil
}

func SaveHistory(connID string, entry HistoryEntry) error {
    entries, err := LoadHistory(connID)
    if err != nil {
        return err
    }

    entry.ID = uuid.New().String()
    entry.ExecutedAt = time.Now()
    entries = append(entries, entry)

    // Evict oldest if over limit
    if len(entries) > maxHistoryEntries {
        sort.Slice(entries, func(i, j int) bool {
            return entries[i].ExecutedAt.Before(entries[j].ExecutedAt)
        })
        entries = entries[len(entries)-maxHistoryEntries:]
    }

    data, err := json.MarshalIndent(entries, "", "  ")
    if err != nil {
        return err
    }

    path, err := historyPath(connID)
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0600)
}

func ClearHistory(connID string) error {
    path, err := historyPath(connID)
    if err != nil {
        return err
    }
    return os.WriteFile(path, []byte("[]"), 0600)
}
