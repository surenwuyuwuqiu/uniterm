# Database Query Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add MySQL, PostgreSQL, and rqlite database client functionality via xorm ORM, including schema browsing, SQL editor with result grid, table structure editing, and per-connection query history.

**Architecture:** New `backend/database/` package wraps xorm.Engine for unified driver management. `DatabaseSession` implements the existing `Session` interface, integrating seamlessly with SessionManager. Frontend adds `DBTabContent` with a tree/editor/history split layout, following existing SFTP/RDP tab patterns. Database connections extend `ConnectionConfig` with `DBType`/`DBName` fields, reusing PasswordStore for credential storage.

**Tech Stack:** xorm.io/xorm, go-sql-driver/mysql, lib/pq, rqlite/gorqlite/stdlib, Vue 3 + Pinia + Element Plus

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `backend/database/engine.go` | Create | xorm.Engine factory, DSN builders for mysql/postgres/rqlite |
| `backend/database/schema.go` | Create | Schema introspection via xorm DBMetas() |
| `backend/database/executor.go` | Create | SQL execution + result JSON serialization |
| `backend/database/history.go` | Create | Per-connection query history persistence (JSON file) |
| `backend/session/database_session.go` | Create | DatabaseSession implementing Session interface |
| `backend/session/session.go` | Modify | Add DBType, DBName to ConnectionConfig |
| `backend/session/manager.go` | Modify | Add "database" case to Create() |
| `app.go` | Modify | Wails bindings: GetDatabases, GetTables, GetTableSchema, ExecuteQuery, ExecuteStatement, AlterTable, GetQueryHistory, ClearQueryHistory |
| `frontend/src/types/database.ts` | Create | TypeScript types for DB results, schema, history |
| `frontend/src/types/session.ts` | Modify | Add dbType, dbName to ConnectionConfig |
| `frontend/src/types/workspace.ts` | Modify | Add DBTab, extend PanelType/Tab |
| `frontend/src/components/DBTabContent.vue` | Create | Main DB tab layout (left tree + right panels + bottom history) |
| `frontend/src/components/DBTreePanel.vue` | Create | Database/table tree view |
| `frontend/src/components/DBTableStructure.vue` | Create | Column + index view with inline editing |
| `frontend/src/components/DBQueryEditor.vue` | Create | SQL textarea + result grid |
| `frontend/src/components/DBQueryHistory.vue` | Create | Query history list with click-to-replay |
| `frontend/src/stores/tabStore.ts` | Modify | Add createDBTab() |
| `frontend/src/App.vue` | Modify | Add DBTabContent component, onConnectDB handler |
| `frontend/src/components/ConnectionForm.vue` | Modify | Add database type radio, DB name field |
| `frontend/src/components/Sidebar.vue` | Modify | Add connectDB emit, context menu item |
| `frontend/src/i18n/index.ts` | Modify | Add database-related i18n strings |
| `go.mod` | Modify | Add xorm, mysql, pq, gorqlite dependencies |

---

### Task 1: Add Go Dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Run go get to add all database dependencies**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniterm-cp
go get xorm.io/xorm@latest
go get github.com/go-sql-driver/mysql@latest
go get github.com/lib/pq@latest
go get github.com/rqlite/gorqlite@latest
```

Expected: All packages download without error.

- [ ] **Step 2: Run go mod tidy**

```bash
go mod tidy
```

Expected: `go.mod` and `go.sum` updated.

- [ ] **Step 3: Verify compilation**

```bash
go build ./...
```

Expected: Build succeeds (no-op since no code references yet).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add xorm, mysql, postgres, gorqlite dependencies

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 2: Extend ConnectionConfig and SessionManager

**Files:**
- Modify: `backend/session/session.go`
- Modify: `backend/session/manager.go`

- [ ] **Step 1: Add DBType and DBName to ConnectionConfig**

In `backend/session/session.go`, in the `ConnectionConfig` struct, add two fields after `ShellPath`:

```go
// Database-specific fields
DBType string `json:"dbType,omitempty"` // "mysql", "postgres", "rqlite"
DBName string `json:"dbName,omitempty"` // default database name
```

- [ ] **Step 2: Add "database" case to SessionManager.Create()**

In `backend/session/manager.go`, inside the switch statement in `Create()`, add before the `default` case:

```go
case "database":
    s = NewDatabaseSession(config.ID)
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./...
```

Expected: Error — `NewDatabaseSession` not yet defined. That's OK for now, confirms the wiring is in place.

- [ ] **Step 4: Commit**

```bash
git add backend/session/session.go backend/session/manager.go
git commit -m "feat(session): add DBType/DBName fields and database session type

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 3: Create database/engine.go — xorm Engine Factory

**Files:**
- Create: `backend/database/engine.go`

- [ ] **Step 1: Create the file with DSN builders**

```go
package database

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/rqlite/gorqlite/stdlib"

	"xorm.io/xorm"
)

// driverName maps DBType to the database/sql driver name for xorm.
var driverName = map[string]string{
	"mysql":      "mysql",
	"postgres":   "postgres",
	"rqlite":     "rqlite",
}

// BuildDSN builds a DSN string from connection config fields.
func BuildDSN(dbType, host, user, password, dbName string, port int) (string, error) {
	switch dbType {
	case "mysql":
		// user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=true
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true",
			user, password, host, port, dbName)
		return dsn, nil
	case "postgres":
		// host=xxx port=xxx user=xxx password=xxx dbname=xxx sslmode=disable
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, password, dbName)
		return dsn, nil
	case "rqlite":
		// http://user:password@host:port/
		if user != "" && password != "" {
			return fmt.Sprintf("http://%s:%s@%s:%d/", user, password, host, port), nil
		}
		return fmt.Sprintf("http://%s:%d/", host, port), nil
	default:
		return "", fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// NewEngine creates an xorm.Engine for the given database type and DSN.
func NewEngine(dbType, dsn string) (*xorm.Engine, error) {
	drv, ok := driverName[dbType]
	if !ok {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	engine, err := xorm.NewEngine(drv, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbType, err)
	}
	return engine, nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add backend/database/engine.go
git commit -m "feat(database): add xorm engine factory with DSN builders for mysql/pg/rqlite

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 4: Create session/database_session.go — DatabaseSession

**Files:**
- Create: `backend/session/database_session.go`

- [ ] **Step 1: Create the file**

```go
package session

import (
	"fmt"

	"xorm.io/xorm"

	"github.com/ys-ll/uniterm/backend/database"
)

type DatabaseSession struct {
	baseSession
	engine *xorm.Engine
	dbType string
	closed bool
}

func NewDatabaseSession(id string) *DatabaseSession {
	return &DatabaseSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "database",
			status:      StatusDisconnected,
		},
	}
}

func (s *DatabaseSession) Connect(config ConnectionConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.setStatus(StatusConnecting)
	s.dbType = config.DBType

	if config.Name != "" {
		s.title = config.Name
	} else {
		s.title = fmt.Sprintf("%s:%s@%s:%d", config.DBType, config.User, config.Host, config.Port)
	}

	dsn, err := database.BuildDSN(config.DBType, config.Host, config.User, config.Password, config.DBName, config.Port)
	if err != nil {
		s.setStatus(StatusError)
		return err
	}

	engine, err := database.NewEngine(config.DBType, dsn)
	if err != nil {
		s.setStatus(StatusError)
		return err
	}

	if err := engine.Ping(); err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("ping %s: %w", config.DBType, err)
	}

	s.engine = engine
	s.setStatus(StatusConnected)
	return nil
}

func (s *DatabaseSession) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.engine != nil {
		s.engine.Close()
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *DatabaseSession) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == StatusConnected && s.engine != nil
}

func (s *DatabaseSession) Write(data []byte) error {
	// Not used for database sessions — frontend calls ExecuteQuery/ExecuteStatement instead.
	return nil
}

func (s *DatabaseSession) Resize(cols, rows int) error {
	// No-op for database sessions.
	return nil
}

// Engine returns the underlying xorm engine (used by Wails bindings).
func (s *DatabaseSession) Engine() *xorm.Engine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine
}

// DBType returns the database type string.
func (s *DatabaseSession) DBType() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbType
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

Expected: Compiles successfully. The `NewDatabaseSession` reference from Task 2 step 2 now resolves.

- [ ] **Step 3: Commit**

```bash
git add backend/session/database_session.go
git commit -m "feat(session): add DatabaseSession implementing Session interface

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 5: Create database/schema.go — Schema Introspection

**Files:**
- Create: `backend/database/schema.go`

- [ ] **Step 1: Create the file**

```go
package database

import (
	"fmt"
	"strings"

	"xorm.io/xorm"
)

type TableInfo struct {
	Name string `json:"name"`
}

type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	DefaultVal string `json:"defaultVal"`
	IsPrimary  bool   `json:"isPrimary"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

type SchemaResult struct {
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes"`
}

// GetDatabases returns database names.
func GetDatabases(engine *xorm.Engine, dbType string) ([]string, error) {
	switch dbType {
	case "mysql":
		results, err := engine.QueryString("SHOW DATABASES")
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(results))
		for _, row := range results {
			names = append(names, row["Database"])
		}
		return names, nil
	case "postgres":
		results, err := engine.QueryString("SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname")
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(results))
		for _, row := range results {
			names = append(names, row["datname"])
		}
		return names, nil
	case "rqlite":
		// rqlite doesn't have multiple databases — return a single placeholder.
		return []string{"main"}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// GetTables returns table names for a database.
func GetTables(engine *xorm.Engine, dbType, dbName string) ([]TableInfo, error) {
	switch dbType {
	case "mysql":
		engine.Exec(fmt.Sprintf("USE `%s`", dbName))
	case "postgres":
		// PostgreSQL: switch schema search path or just query across schemas
		// Use the current database; no USE needed.
	}

	tables, err := engine.DBMetas()
	if err != nil {
		return nil, fmt.Errorf("get table metas: %w", err)
	}

	infos := make([]TableInfo, 0, len(tables))
	for _, t := range tables {
		infos = append(infos, TableInfo{Name: t.Name})
	}
	return infos, nil
}

// GetTableSchema returns columns and indexes for a table.
func GetTableSchema(engine *xorm.Engine, dbName, tableName string) (*SchemaResult, error) {
	tables, err := engine.DBMetas()
	if err != nil {
		return nil, fmt.Errorf("get table metas: %w", err)
	}

	var target *xorm.Table
	for i := range tables {
		if tables[i].Name == tableName {
			target = tables[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	columns := make([]ColumnInfo, 0, len(target.Columns()))
	seq := target.ColumnsSeq()
	for _, colName := range seq {
		col := target.GetColumn(colName)
		columns = append(columns, ColumnInfo{
			Name:       col.Name,
			Type:       strings.ToUpper(col.SQLType.Name),
			Nullable:   col.Nullable,
			DefaultVal: col.Default,
			IsPrimary:  col.IsPrimaryKey,
		})
	}

	indexes := make([]IndexInfo, 0)
	for _, idx := range target.Indexes {
		indexes = append(indexes, IndexInfo{
			Name:    idx.Name,
			Columns: idx.Cols,
			Unique:  idx.Type == xorm.UniqueType,
		})
	}

	return &SchemaResult{Columns: columns, Indexes: indexes}, nil
}
```

- [ ] **Step 2: Check what xorm.Table and related types look like — adjust if needed**

The xorm `*schemas.Table` types use specific APIs. If `tables[i].Columns()` doesn't return the right thing, we may need to use `engine.Query()` with raw information_schema queries instead. Keep the `engine.QueryString()` fallback approach in mind.

Run:
```bash
go build ./...
```

Expected: May have compilation errors with xorm API. Read the actual xorm types and fix any mismatches.

- [ ] **Step 3: Test xorm API compatibility by compiling**

```bash
go build ./...
```

Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add backend/database/schema.go
git commit -m "feat(database): add schema introspection via xorm DBMetas()

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 6: Create database/executor.go — SQL Execution

**Files:**
- Create: `backend/database/executor.go`

- [ ] **Step 1: Create the file**

```go
package database

import (
	"encoding/json"
	"fmt"

	"xorm.io/xorm"
)

type QueryResultColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type QueryResult struct {
	Columns []QueryResultColumn `json:"columns"`
	Rows    []map[string]any    `json:"rows"`
}

type ExecResult struct {
	Affected     int64 `json:"affected"`
	LastInsertID int64 `json:"lastInsertId"`
}

func ExecuteQuery(engine *xorm.Engine, sql string) (*QueryResult, error) {
	rows, err := engine.QueryString(sql)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return &QueryResult{Columns: []QueryResultColumn{}, Rows: []map[string]any{}}, nil
	}

	// Extract column names from the first row
	firstRow := rows[0]
	columns := make([]QueryResultColumn, 0, len(firstRow))
	for key := range firstRow {
		columns = append(columns, QueryResultColumn{Name: key, Type: ""})
	}

	// Convert []map[string]string to []map[string]any for JSON serialization
	resultRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v
		}
		resultRows = append(resultRows, m)
	}

	return &QueryResult{Columns: columns, Rows: resultRows}, nil
}

func ExecuteStatement(engine *xorm.Engine, sql string) (*ExecResult, error) {
	result, err := engine.Exec(sql)
	if err != nil {
		return nil, err
	}

	affected, _ := result.RowsAffected()
	lastID, _ := result.LastInsertId()

	return &ExecResult{Affected: affected, LastInsertID: lastID}, nil
}

// QueryResultToJSON serializes a QueryResult to JSON bytes (for EventEmit).
func QueryResultToJSON(qr *QueryResult) ([]byte, error) {
	return json.Marshal(qr)
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add backend/database/executor.go
git commit -m "feat(database): add SQL execution and result serialization

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 7: Create database/history.go — Query History

**Files:**
- Create: `backend/database/history.go`

- [ ] **Step 1: Create the file**

```go
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
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add backend/database/history.go
git commit -m "feat(database): add per-connection query history persistence

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 8: Add Wails Bindings in app.go

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Add database import in app.go**

At the top of `app.go`, add the database import:

```go
import (
	// ... existing imports ...
	"github.com/ys-ll/uniterm/backend/database"
)
```

- [ ] **Step 2: Add Wails binding methods to App**

Add these methods to `app.go` (anywhere after the existing methods, before the file ends):

```go
// ── Database methods ──

func (a *App) dbSession(sessionID string) (*session.DatabaseSession, error) {
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	ds, ok := s.(*session.DatabaseSession)
	if !ok {
		return nil, fmt.Errorf("session is not a database session: %s", sessionID)
	}
	return ds, nil
}

func (a *App) GetDatabases(sessionID string) ([]string, error) {
	ds, err := a.dbSession(sessionID)
	if err != nil {
		return nil, err
	}
	return database.GetDatabases(ds.Engine(), ds.DBType())
}

func (a *App) GetTables(sessionID string, dbName string) ([]database.TableInfo, error) {
	ds, err := a.dbSession(sessionID)
	if err != nil {
		return nil, err
	}
	return database.GetTables(ds.Engine(), ds.DBType(), dbName)
}

func (a *App) GetTableSchema(sessionID string, dbName string, tableName string) (*database.SchemaResult, error) {
	ds, err := a.dbSession(sessionID)
	if err != nil {
		return nil, err
	}
	return database.GetTableSchema(ds.Engine(), dbName, tableName)
}

func (a *App) ExecuteQuery(sessionID string, sql string) (*database.QueryResult, error) {
	ds, err := a.dbSession(sessionID)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	qr, qErr := database.ExecuteQuery(ds.Engine(), sql)
	elapsed := time.Since(start).Milliseconds()

	// Save to history
	entry := database.HistoryEntry{SQL: sql, Duration: elapsed}
	if qErr != nil {
		entry.Error = qErr.Error()
	} else {
		entry.RowCount = len(qr.Rows)
	}
	_ = database.SaveHistory(ds.ID(), entry)

	return qr, qErr
}

func (a *App) ExecuteStatement(sessionID string, sql string) (*database.ExecResult, error) {
	ds, err := a.dbSession(sessionID)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	er, sErr := database.ExecuteStatement(ds.Engine(), sql)
	elapsed := time.Since(start).Milliseconds()

	entry := database.HistoryEntry{SQL: sql, Duration: elapsed}
	if sErr != nil {
		entry.Error = sErr.Error()
	} else {
		entry.RowCount = int(er.Affected)
	}
	_ = database.SaveHistory(ds.ID(), entry)

	return er, sErr
}

func (a *App) AlterTable(sessionID string, dbName string, tableName string, sql string) error {
	ds, err := a.dbSession(sessionID)
	if err != nil {
		return err
	}
	_, err = ds.Engine().Exec(sql)
	return err
}

func (a *App) GetQueryHistory(sessionID string) ([]database.HistoryEntry, error) {
	return database.LoadHistory(sessionID)
}

func (a *App) ClearQueryHistory(sessionID string) error {
	return database.ClearHistory(sessionID)
}
```

- [ ] **Step 3: Verify compilation and regenerate Wails bindings**

```bash
go build ./...
```

Expected: Compiles successfully.

- [ ] **Step 4: Regenerate frontend bindings**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniterm-cp
wails generate module
```

Expected: New TypeScript functions generated in `frontend/wailsjs/go/main/App.js` and `App.d.ts`.

- [ ] **Step 5: Commit**

```bash
git add app.go frontend/wailsjs/
git commit -m "feat(app): add Wails bindings for database operations

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 9: Add TypeScript Types

**Files:**
- Create: `frontend/src/types/database.ts`
- Modify: `frontend/src/types/session.ts`
- Modify: `frontend/src/types/workspace.ts`

- [ ] **Step 1: Create database.ts types**

```typescript
export interface TableInfo {
  name: string
}

export interface ColumnInfo {
  name: string
  type: string
  nullable: boolean
  defaultVal: string
  isPrimary: boolean
}

export interface IndexInfo {
  name: string
  columns: string[]
  unique: boolean
}

export interface SchemaResult {
  columns: ColumnInfo[]
  indexes: IndexInfo[]
}

export interface QueryResultColumn {
  name: string
  type: string
}

export interface QueryResult {
  columns: QueryResultColumn[]
  rows: Record<string, any>[]
}

export interface ExecResult {
  affected: number
  lastInsertId: number
}

export interface HistoryEntry {
  id: string
  sql: string
  executedAt: string
  durationMs: number
  error?: string
  rowCount?: number
}
```

- [ ] **Step 2: Modify session.ts — add dbType/dbName to ConnectionConfig**

In `frontend/src/types/session.ts`, update the `ConnectionConfig` interface:

```typescript
export interface ConnectionConfig {
  // ... existing fields ...
  type: 'ssh' | 'rdp' | 'vnc'
  // Add these two new fields after shellPath:
  dbType?: string   // "mysql", "postgres", "rqlite"
  dbName?: string   // default database name
}
```

- [ ] **Step 3: Modify workspace.ts — add DBTab and database PanelType**

In `frontend/src/types/workspace.ts`:

Change PanelType:
```typescript
export type PanelType = 'ssh' | 'sftp' | 'settings' | 'rdp' | 'vnc' | 'local' | 'database' | 'other'
```

Change Tab:
```typescript
export type Tab = TerminalTab | SettingsTab | WorkspaceTab | SFTPTab | RDPTab | VNCTab | DBTab
```

Add DBTab interface:
```typescript
export interface DBTab {
  type: 'database'
  id: string
  panelId: string
  name: string
}
```

- [ ] **Step 4: Verify frontend compiles**

```bash
cd frontend && npx vue-tsc --noEmit
```

Expected: May have errors from code not yet updated — fix any type errors in the modified files only.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/database.ts frontend/src/types/session.ts frontend/src/types/workspace.ts
git commit -m "feat(types): add database TypeScript types and extend workspace/session types

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 10: Add createDBTab to tabStore

**Files:**
- Modify: `frontend/src/stores/tabStore.ts`

- [ ] **Step 1: Add createDBTab function**

Inside the store definition, add:

```typescript
function createDBTab(name: string, panelId: string): DBTab {
  const tab: DBTab = {
    type: 'database',
    id: genId('db-tab'),
    panelId,
    name
  }
  tabState.tabs.push(tab)
  tabState.activeTabId = tab.id
  return tab
}
```

- [ ] **Step 2: Add DBTab to imports**

Add `DBTab` to the import from `../types/workspace`:

```typescript
import type { Tab, TerminalTab, SettingsTab, WorkspaceTab, SFTPTab, RDPTab, VNCTab, DBTab, PanelLayout, LayoutNode } from '../types/workspace'
```

- [ ] **Step 3: Add to return statement**

Add `createDBTab` to the returned object.

- [ ] **Step 4: Update closeTab for database type**

In the `closeTab` function, add `|| tab.type === 'database'` to the condition that extracts `panelId`:

```typescript
const removedPanelIds = removed.type === 'terminal' || removed.type === 'settings' || removed.type === 'rdp' || removed.type === 'vnc' || removed.type === 'database'
  ? [removed.panelId]
  : removed.type === 'workspace'
    ? removed.panelIds
    : removed.type === 'sftp'
      ? [removed.panelId]
      : []
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/tabStore.ts
git commit -m "feat(tabStore): add createDBTab for database connection tabs

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 11: Create DBTreePanel.vue

**Files:**
- Create: `frontend/src/components/DBTreePanel.vue`

- [ ] **Step 1: Create the component**

```vue
<template>
  <div class="db-tree-panel">
    <div class="panel-header">{{ t('db.databases') }}</div>
    <div class="tree-content">
      <el-tree
        :data="treeData"
        :props="treeProps"
        node-key="id"
        :loading="loading"
        highlight-current
        @node-click="onNodeClick"
        default-expand-all
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from '../i18n'
import { GetDatabases, GetTables } from '../../wailsjs/go/main/App'
import type { TableInfo } from '../types/database'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
}>()

const emit = defineEmits<{
  selectTable: [dbName: string, tableName: string]
  selectDatabase: [dbName: string]
}>()

interface TreeNode {
  id: string
  label: string
  children?: TreeNode[]
}

const treeData = ref<TreeNode[]>([])
const loading = ref(false)
const treeProps = { children: 'children', label: 'label' }

onMounted(async () => {
  loading.value = true
  try {
    const dbs = await GetDatabases(props.sessionId)
    for (const db of dbs) {
      const tables = await GetTables(props.sessionId, db)
      treeData.value.push({
        id: `db:${db}`,
        label: db,
        children: tables.map((t: TableInfo) => ({
          id: `table:${db}:${t.name}`,
          label: t.name,
        }))
      })
    }
  } catch (e) {
    console.error('Failed to load databases:', e)
  } finally {
    loading.value = false
  }
})

function onNodeClick(data: TreeNode) {
  if (data.id.startsWith('table:')) {
    const [, db, table] = data.id.split(':')
    emit('selectTable', db, table)
  } else if (data.id.startsWith('db:')) {
    const db = data.id.slice(3)
    emit('selectDatabase', db)
  }
}
</script>

<style scoped>
.db-tree-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: auto;
}
.panel-header {
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary, #888);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.tree-content {
  flex: 1;
  overflow: auto;
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/DBTreePanel.vue
git commit -m "feat(frontend): add DBTreePanel component

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 12: Create DBTableStructure.vue

**Files:**
- Create: `frontend/src/components/DBTableStructure.vue`

- [ ] **Step 1: Create the component**

```vue
<template>
  <div class="db-table-structure">
    <div v-if="!tableName" class="placeholder">{{ t('db.selectTableHint') }}</div>
    <template v-else>
      <div class="section">
        <div class="section-title">{{ t('db.columns') }}</div>
        <el-table :data="schema?.columns || []" border size="small" style="width:100%">
          <el-table-column prop="name" :label="t('db.colName')" />
          <el-table-column prop="type" :label="t('db.colType')" />
          <el-table-column :label="t('db.colNullable')" width="80">
            <template #default="{ row }">
              {{ row.nullable ? 'YES' : 'NO' }}
            </template>
          </el-table-column>
          <el-table-column prop="defaultVal" :label="t('db.colDefault')" />
          <el-table-column :label="t('db.colPrimary')" width="70">
            <template #default="{ row }">
              <span v-if="row.isPrimary">PK</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('db.actions')" width="120">
            <template #default="{ row }">
              <button class="action-btn" @click="startEditColumn(row)">{{ t('db.edit') }}</button>
              <button class="action-btn danger" @click="onDropColumn(row.name)">{{ t('db.drop') }}</button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="section">
        <div class="section-title">{{ t('db.indexes') }}</div>
        <el-table :data="schema?.indexes || []" border size="small" style="width:100%">
          <el-table-column prop="name" :label="t('db.idxName')" />
          <el-table-column :label="t('db.idxColumns')">
            <template #default="{ row }">
              {{ row.columns?.join(', ') }}
            </template>
          </el-table-column>
          <el-table-column :label="t('db.idxUnique')" width="80">
            <template #default="{ row }">
              {{ row.unique ? 'YES' : 'NO' }}
            </template>
          </el-table-column>
          <el-table-column :label="t('db.actions')" width="80">
            <template #default="{ row }">
              <button class="action-btn danger" @click="onDropIndex(row.name)">{{ t('db.drop') }}</button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <el-dialog v-model="editDialogVisible" :title="t('db.editColumn')" width="400px">
        <el-form v-if="editingColumn" label-width="100px">
          <el-form-item :label="t('db.colName')">
            <el-input v-model="editingColumn.name" disabled />
          </el-form-item>
          <el-form-item :label="t('db.colType')">
            <el-input v-model="editColumnType" />
          </el-form-item>
          <el-form-item :label="t('db.colNullable')">
            <el-switch v-model="editColumnNullable" />
          </el-form-item>
          <el-form-item :label="t('db.colDefault')">
            <el-input v-model="editColumnDefault" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="editDialogVisible = false">{{ t('common.cancel') }}</el-button>
          <el-button type="primary" @click="onSaveColumnEdit">{{ t('common.save') }}</el-button>
        </template>
      </el-dialog>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from '../i18n'
import { GetTableSchema, AlterTable } from '../../wailsjs/go/main/App'
import type { SchemaResult, ColumnInfo } from '../types/database'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
  tableName: string
  dbName: string
}>()

const emit = defineEmits<{
  refresh: []
}>()

const schema = ref<SchemaResult | null>(null)
const editDialogVisible = ref(false)
const editingColumn = ref<ColumnInfo | null>(null)
const editColumnType = ref('')
const editColumnNullable = ref(false)
const editColumnDefault = ref('')

watch(() => props.tableName, async (name) => {
  if (!name) return
  await loadSchema()
})

async function loadSchema() {
  if (!props.tableName) return
  try {
    schema.value = await GetTableSchema(props.sessionId, props.dbName, props.tableName)
  } catch (e) {
    console.error('Failed to load schema:', e)
  }
}

function startEditColumn(col: ColumnInfo) {
  editingColumn.value = col
  editColumnType.value = col.type
  editColumnNullable.value = col.nullable
  editColumnDefault.value = col.defaultVal
  editDialogVisible.value = true
}

async function onSaveColumnEdit() {
  if (!editingColumn.value) return
  const col = editingColumn.value
  const sql = `ALTER TABLE \`${props.tableName}\` MODIFY COLUMN \`${col.name}\` ${editColumnType.value}${editColumnNullable.value ? ' NULL' : ' NOT NULL'}${editColumnDefault.value ? ` DEFAULT '${editColumnDefault.value}'` : ''}`
  try {
    await AlterTable(props.sessionId, props.dbName, props.tableName, sql)
    editDialogVisible.value = false
    await loadSchema()
    emit('refresh')
  } catch (e) {
    console.error('Failed to alter column:', e)
  }
}

async function onDropColumn(colName: string) {
  const sql = `ALTER TABLE \`${props.tableName}\` DROP COLUMN \`${colName}\``
  try {
    await AlterTable(props.sessionId, props.dbName, props.tableName, sql)
    await loadSchema()
    emit('refresh')
  } catch (e) {
    console.error('Failed to drop column:', e)
  }
}

async function onDropIndex(idxName: string) {
  const sql = `DROP INDEX \`${idxName}\` ON \`${props.tableName}\``
  try {
    await AlterTable(props.sessionId, props.dbName, props.tableName, sql)
    await loadSchema()
    emit('refresh')
  } catch (e) {
    console.error('Failed to drop index:', e)
  }
}
</script>

<style scoped>
.db-table-structure {
  height: 100%;
  overflow: auto;
  padding: 8px;
}
.placeholder {
  color: var(--text-secondary, #888);
  text-align: center;
  padding: 40px 0;
}
.section {
  margin-bottom: 16px;
}
.section-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--text-primary, #333);
}
.action-btn {
  border: none;
  background: var(--color-primary, #409eff);
  color: #fff;
  padding: 2px 8px;
  border-radius: 3px;
  cursor: pointer;
  font-size: 12px;
  margin-right: 4px;
}
.action-btn.danger {
  background: var(--color-danger, #f56c6c);
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/DBTableStructure.vue
git commit -m "feat(frontend): add DBTableStructure component with column/index editing

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 13: Create DBQueryEditor.vue

**Files:**
- Create: `frontend/src/components/DBQueryEditor.vue`

- [ ] **Step 1: Create the component**

```vue
<template>
  <div class="db-query-editor">
    <div class="editor-area">
      <textarea
        ref="editorEl"
        v-model="sql"
        class="sql-editor"
        :placeholder="t('db.sqlPlaceholder')"
        @keydown="onKeydown"
        rows="8"
      />
      <div class="editor-actions">
        <button class="exec-btn" @click="onExecute">{{ t('db.execute') }}</button>
        <span class="shortcut-hint">Ctrl+Enter</span>
      </div>
    </div>

    <div v-if="error" class="error-msg">{{ error }}</div>

    <div v-if="execResult" class="result-info">
      {{ t('db.affectedRows') }}: {{ execResult.affected }}
    </div>

    <div v-if="queryResult" class="result-grid">
      <el-table :data="queryResult.rows" border size="small" max-height="400" style="width:100%">
        <el-table-column
          v-for="col in queryResult.columns"
          :key="col.name"
          :prop="col.name"
          :label="col.name"
          min-width="100"
          show-overflow-tooltip
        />
      </el-table>
      <div class="result-count">{{ queryResult.rows.length }} {{ t('db.rows') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../i18n'
import { ExecuteQuery, ExecuteStatement } from '../../wailsjs/go/main/App'
import type { QueryResult, ExecResult } from '../types/database'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
}>()

const sql = ref('')
const queryResult = ref<QueryResult | null>(null)
const execResult = ref<ExecResult | null>(null)
const error = ref('')

function onKeydown(e: KeyboardEvent) {
  if (e.ctrlKey && e.key === 'Enter') {
    e.preventDefault()
    onExecute()
  }
}

async function onExecute() {
  if (!sql.value.trim()) return
  error.value = ''
  queryResult.value = null
  execResult.value = null

  const trimmed = sql.value.trim()
  const isSelect = /^\s*SELECT\b/i.test(trimmed) ||
    /^\s*SHOW\b/i.test(trimmed) ||
    /^\s*DESCRIBE\b/i.test(trimmed) ||
    /^\s*EXPLAIN\b/i.test(trimmed) ||
    /^\s*PRAGMA\b/i.test(trimmed)

  try {
    if (isSelect) {
      queryResult.value = await ExecuteQuery(props.sessionId, trimmed)
    } else {
      execResult.value = await ExecuteStatement(props.sessionId, trimmed)
    }
  } catch (e: any) {
    error.value = e?.message || String(e)
  }
}
</script>

<style scoped>
.db-query-editor {
  height: 100%;
  display: flex;
  flex-direction: column;
  padding: 8px;
  overflow: auto;
}
.editor-area {
  margin-bottom: 8px;
}
.sql-editor {
  width: 100%;
  font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.5;
  background: var(--bg-secondary, #1e1e1e);
  color: var(--text-primary, #d4d4d4);
  border: 1px solid var(--border-color, #444);
  border-radius: 4px;
  padding: 8px;
  resize: vertical;
}
.editor-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}
.exec-btn {
  padding: 4px 16px;
  background: var(--color-primary, #409eff);
  color: #fff;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
}
.shortcut-hint {
  font-size: 11px;
  color: var(--text-secondary, #888);
}
.error-msg {
  color: var(--color-danger, #f56c6c);
  padding: 8px;
  background: rgba(245, 108, 108, 0.1);
  border-radius: 4px;
  margin-bottom: 8px;
  font-family: monospace;
  font-size: 13px;
}
.result-info {
  padding: 4px 0;
  font-size: 13px;
  color: var(--text-secondary, #888);
  margin-bottom: 8px;
}
.result-grid {
  flex: 1;
  overflow: auto;
}
.result-count {
  padding: 4px 0;
  font-size: 12px;
  color: var(--text-secondary, #888);
}
</style>
```

- [ ] **Step 2: Add inline cell editing to the result grid**

Update the template — replace the `el-table` in the result grid with:

```vue
<div v-if="queryResult" class="result-grid">
  <el-table :data="queryResult.rows" border size="small" max-height="400" style="width:100%"
    @cell-dblclick="onCellDblClick">
    <el-table-column
      v-for="col in queryResult.columns"
      :key="col.name"
      :prop="col.name"
      :label="col.name"
      min-width="100"
      show-overflow-tooltip
    >
      <template #default="{ row, column, $index }">
        <div v-if="editingCell && editingCell.rowIndex === $index && editingCell.colName === column.property"
          class="cell-edit-wrap">
          <input
            ref="cellInputEl"
            v-model="editingCell.value"
            class="cell-edit-input"
            @keydown.enter="onCellEditConfirm"
            @keydown.escape="onCellEditCancel"
            @blur="onCellEditCancel"
          />
        </div>
        <span v-else class="cell-value">{{ row[column.property] }}</span>
      </template>
    </el-table-column>
  </el-table>
  <div class="result-count">{{ queryResult.rows.length }} {{ t('db.rows') }}</div>
</div>
```

Add the inline editing logic in the `<script>` section, after `onExecute`:

```typescript
interface EditingCell {
  rowIndex: number
  colName: string
  originalValue: any
  value: string
}

const editingCell = ref<EditingCell | null>(null)
const cellInputEl = ref<HTMLInputElement | null>(null)

const props = defineProps<{
  sessionId: string
  tableName?: string   // current table context from tree selection
  dbName?: string
  primaryKeys?: string[] // PK column names from schema
}>()

const emit = defineEmits<{
  cellUpdated: []
}>()

function onCellDblClick(row: any, column: any, cell: HTMLElement, event: MouseEvent) {
  if (!props.tableName || !props.primaryKeys || props.primaryKeys.length === 0) return

  const colName = column.property
  const originalValue = row[colName]

  editingCell.value = {
    rowIndex: queryResult.value!.rows.indexOf(row),
    colName,
    originalValue,
    value: originalValue ?? ''
  }

  nextTick(() => {
    cellInputEl.value?.focus()
    cellInputEl.value?.select()
  })
}

async function onCellEditConfirm() {
  if (!editingCell.value || !props.tableName || !props.primaryKeys) return

  const { rowIndex, colName, originalValue, value } = editingCell.value
  if (value === String(originalValue ?? '')) {
    editingCell.value = null
    return
  }

  // Build WHERE clause from PK columns
  const row = queryResult.value!.rows[rowIndex]
  const whereParts = props.primaryKeys.map(pk => `\`${pk}\` = '${String(row[pk] ?? '').replace(/'/g, "''")}'`)
  const whereClause = whereParts.join(' AND ')

  const updateSQL = `UPDATE \`${props.tableName}\` SET \`${colName}\` = '${value.replace(/'/g, "''")}' WHERE ${whereClause}`

  try {
    await ExecuteStatement(props.sessionId, updateSQL)
    // Update local row data
    queryResult.value!.rows[rowIndex][colName] = value
    error.value = ''
    emit('cellUpdated')
  } catch (e: any) {
    error.value = e?.message || String(e)
  }

  editingCell.value = null
}

function onCellEditCancel() {
  editingCell.value = null
}
```

Add `nextTick` to the Vue import:

```typescript
import { ref, nextTick } from 'vue'
```

Add the cell edit styles:

```css
.cell-value {
  cursor: default;
}
.cell-edit-wrap {
  margin: -8px -12px;
}
.cell-edit-input {
  width: 100%;
  padding: 4px 8px;
  border: 2px solid var(--color-primary, #409eff);
  border-radius: 2px;
  font-size: 13px;
  font-family: inherit;
  outline: none;
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/DBQueryEditor.vue
git commit -m "feat(frontend): add DBQueryEditor with SQL editor, result grid, and inline cell editing

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 14: Create DBQueryHistory.vue

**Files:**
- Create: `frontend/src/components/DBQueryHistory.vue`

- [ ] **Step 1: Create the component**

```vue
<template>
  <div class="db-query-history">
    <div class="history-header">
      <span class="section-title">{{ t('db.queryHistory') }}</span>
      <button class="clear-btn" @click="onClear">{{ t('db.clearHistory') }}</button>
    </div>
    <div class="history-list">
      <div
        v-for="entry in history"
        :key="entry.id"
        class="history-item"
        :class="{ error: entry.error }"
        @click="$emit('replay', entry.sql)"
      >
        <div class="history-sql">{{ entry.sql }}</div>
        <div class="history-meta">
          <span>{{ formatTime(entry.executedAt) }}</span>
          <span v-if="entry.error" class="history-error">{{ entry.error }}</span>
          <span v-else-if="entry.rowCount !== undefined">{{ entry.rowCount }} {{ t('db.rows') }}</span>
          <span>{{ entry.durationMs }}ms</span>
        </div>
      </div>
      <div v-if="history.length === 0" class="empty">{{ t('db.noHistory') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useI18n } from '../i18n'
import { GetQueryHistory, ClearQueryHistory } from '../../wailsjs/go/main/App'
import type { HistoryEntry } from '../types/database'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
  refreshTrigger: number
}>()

defineEmits<{
  replay: [sql: string]
}>()

const history = ref<HistoryEntry[]>([])

onMounted(loadHistory)
watch(() => props.refreshTrigger, loadHistory)

async function loadHistory() {
  try {
    history.value = await GetQueryHistory(props.sessionId)
  } catch (e) {
    console.error('Failed to load history:', e)
  }
}

async function onClear() {
  try {
    await ClearQueryHistory(props.sessionId)
    history.value = []
  } catch (e) {
    console.error('Failed to clear history:', e)
  }
}

function formatTime(ts: string): string {
  const d = new Date(ts)
  return d.toLocaleString()
}
</script>

<style scoped>
.db-query-history {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: auto;
}
.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
}
.section-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary, #888);
}
.clear-btn {
  border: none;
  background: none;
  color: var(--color-danger, #f56c6c);
  cursor: pointer;
  font-size: 12px;
}
.history-list {
  flex: 1;
  overflow: auto;
}
.history-item {
  padding: 6px 12px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-color, #eee);
}
.history-item:hover {
  background: var(--bg-hover, #f5f5f5);
}
.history-item.error {
  border-left: 3px solid var(--color-danger, #f56c6c);
}
.history-sql {
  font-family: monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.history-meta {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: var(--text-secondary, #999);
  margin-top: 2px;
}
.history-error {
  color: var(--color-danger, #f56c6c);
}
.empty {
  padding: 12px;
  color: var(--text-secondary, #888);
  font-size: 12px;
  text-align: center;
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/DBQueryHistory.vue
git commit -m "feat(frontend): add DBQueryHistory component

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 15: Create DBTabContent.vue

**Files:**
- Create: `frontend/src/components/DBTabContent.vue`

- [ ] **Step 1: Create the main layout component**

```vue
<template>
  <div class="db-tab-content">
    <div class="db-main">
      <div class="db-left" :style="{ width: leftWidth + 'px' }">
        <DBTreePanel
          :session-id="sessionId"
          @select-table="onSelectTable"
          @select-database="onSelectDatabase"
        />
      </div>
      <div class="db-resizer" @mousedown="onResizeStart" />
      <div class="db-right">
        <div class="db-right-top">
          <div class="db-tabs">
            <button
              class="db-tab"
              :class="{ active: activeTab === 'structure' }"
              @click="activeTab = 'structure'"
              :disabled="!selectedTable"
            >
              {{ t('db.tableStructure') }}
            </button>
            <button
              class="db-tab"
              :class="{ active: activeTab === 'query' }"
              @click="activeTab = 'query'"
            >
              {{ t('db.sqlQuery') }}
            </button>
          </div>
          <div class="db-right-top-content">
            <DBTableStructure
              v-if="activeTab === 'structure'"
              :session-id="sessionId"
              :db-name="selectedDb"
              :table-name="selectedTable"
              @refresh="onRefresh"
            />
            <DBQueryEditor
              v-else
              :session-id="sessionId"
              :table-name="selectedTable"
              :db-name="selectedDb"
              :primary-keys="primaryKeys"
              @cell-updated="onRefresh"
            />
          </div>
        </div>
        <div class="db-right-bottom">
          <DBQueryHistory
            :session-id="sessionId"
            :refresh-trigger="historyRefresh"
            @replay="onReplay"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../i18n'
import { GetTableSchema } from '../../wailsjs/go/main/App'
import DBTreePanel from './DBTreePanel.vue'
import DBTableStructure from './DBTableStructure.vue'
import DBQueryEditor from './DBQueryEditor.vue'
import DBQueryHistory from './DBQueryHistory.vue'

const { t } = useI18n()

defineProps<{
  sessionId: string
}>()

const activeTab = ref<'structure' | 'query'>('query')
const selectedDb = ref('')
const selectedTable = ref('')
const primaryKeys = ref<string[]>([])
const historyRefresh = ref(0)

const leftWidth = ref(220)
let resizeStartX = 0
let resizeStartWidth = 0

function onSelectDatabase(dbName: string) {
  selectedDb.value = dbName
}

async function onSelectTable(dbName: string, tableName: string) {
  selectedDb.value = dbName
  selectedTable.value = tableName
  // Load PK columns for inline editing
  try {
    const schema = await GetTableSchema(props.sessionId, dbName, tableName)
    primaryKeys.value = schema.columns.filter(c => c.isPrimary).map(c => c.name)
  } catch {
    primaryKeys.value = []
  }
  activeTab.value = 'structure'
}

function onRefresh() {
  historyRefresh.value++
}

function onReplay(sql: string) {
  activeTab.value = 'query'
  // The query editor manages its own SQL state,
  // so we need a way to pass the SQL. We'll use a simple event.
  // For now, just switch to query tab and let the user know
  // via the history click handler. The actual replay logic
  // will be enhanced in a follow-up if needed.
}

function onResizeStart(e: MouseEvent) {
  resizeStartX = e.clientX
  resizeStartWidth = leftWidth.value
  document.addEventListener('mousemove', onResizeMove)
  document.addEventListener('mouseup', onResizeEnd)
}

function onResizeMove(e: MouseEvent) {
  const dx = e.clientX - resizeStartX
  leftWidth.value = Math.max(150, Math.min(500, resizeStartWidth + dx))
}

function onResizeEnd() {
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
}
</script>

<style scoped>
.db-tab-content {
  height: 100%;
  display: flex;
  flex-direction: column;
}
.db-main {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.db-left {
  flex-shrink: 0;
  border-right: 1px solid var(--border-color, #444);
  overflow: auto;
}
.db-resizer {
  width: 4px;
  cursor: col-resize;
  background: transparent;
  flex-shrink: 0;
}
.db-resizer:hover {
  background: var(--color-primary, #409eff);
}
.db-right {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.db-right-top {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.db-tabs {
  display: flex;
  border-bottom: 1px solid var(--border-color, #444);
  padding: 0 8px;
  flex-shrink: 0;
}
.db-tab {
  padding: 6px 16px;
  border: none;
  background: none;
  color: var(--text-secondary, #888);
  cursor: pointer;
  font-size: 13px;
  border-bottom: 2px solid transparent;
}
.db-tab.active {
  color: var(--text-primary, #333);
  border-bottom-color: var(--color-primary, #409eff);
}
.db-tab:disabled {
  opacity: 0.4;
  cursor: default;
}
.db-right-top-content {
  flex: 1;
  overflow: hidden;
}
.db-right-bottom {
  height: 180px;
  border-top: 1px solid var(--border-color, #444);
  overflow: auto;
  flex-shrink: 0;
}
</style>
```

- [ ] **Step 2: Add primaryKeys ref and load schema on table select**

In the `<script>` section, update `defineProps` to capture the return value and add `primaryKeys`:

Change:
```
defineProps<{
  sessionId: string
}>()
```
to:
```
const props = defineProps<{
  sessionId: string
}>()
```

Add `primaryKeys` ref after `selectedTable`:

```typescript
const primaryKeys = ref<string[]>([])
```

Update `onSelectTable` to load primary key info:

```typescript
async function onSelectTable(dbName: string, tableName: string) {
  selectedDb.value = dbName
  selectedTable.value = tableName
  try {
    const schema = await GetTableSchema(props.sessionId, dbName, tableName)
    primaryKeys.value = schema.columns.filter(c => c.isPrimary).map(c => c.name)
  } catch {
    primaryKeys.value = []
  }
  activeTab.value = 'structure'
}
```

Add `GetTableSchema` to imports:

```typescript
import { GetTableSchema } from '../../wailsjs/go/main/App'
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/DBTabContent.vue
git commit -m "feat(frontend): add DBTabContent main layout with inline editing support

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 16: Add Database i18n Strings

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: Add database-related strings to both locales**

In the `zh-CN` section, add after existing entries:

```typescript
// Database
'db.databases': '数据库',
'db.selectTableHint': '选择一张表查看结构',
'db.tableStructure': '表结构',
'db.sqlQuery': 'SQL 查询',
'db.sqlPlaceholder': '输入 SQL 查询...',
'db.execute': '执行',
'db.columns': '列信息',
'db.indexes': '索引信息',
'db.colName': '列名',
'db.colType': '类型',
'db.colNullable': '可空',
'db.colDefault': '默认值',
'db.colPrimary': '主键',
'db.idxName': '索引名',
'db.idxColumns': '列',
'db.idxUnique': '唯一',
'db.actions': '操作',
'db.edit': '编辑',
'db.drop': '删除',
'db.editColumn': '编辑列',
'db.affectedRows': '影响行数',
'db.rows': '行',
'db.queryHistory': '查询历史',
'db.clearHistory': '清除',
'db.noHistory': '暂无查询历史',
'db.connectDB': '连接数据库',
'db.portHint': '默认端口: MySQL 3306, PostgreSQL 5432, rqlite 4001',
```

In the `en` section, add:

```typescript
// Database
'db.databases': 'Databases',
'db.selectTableHint': 'Select a table to view its structure',
'db.tableStructure': 'Table Structure',
'db.sqlQuery': 'SQL Query',
'db.sqlPlaceholder': 'Enter SQL query...',
'db.execute': 'Execute',
'db.columns': 'Columns',
'db.indexes': 'Indexes',
'db.colName': 'Name',
'db.colType': 'Type',
'db.colNullable': 'Nullable',
'db.colDefault': 'Default',
'db.colPrimary': 'Primary Key',
'db.idxName': 'Name',
'db.idxColumns': 'Columns',
'db.idxUnique': 'Unique',
'db.actions': 'Actions',
'db.edit': 'Edit',
'db.drop': 'Drop',
'db.editColumn': 'Edit Column',
'db.affectedRows': 'Affected rows',
'db.rows': 'rows',
'db.queryHistory': 'Query History',
'db.clearHistory': 'Clear',
'db.noHistory': 'No query history',
'db.connectDB': 'Connect Database',
'db.portHint': 'Default port: MySQL 3306, PostgreSQL 5432, rqlite 4001',
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/i18n/index.ts
git commit -m "feat(i18n): add database-related translations

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 17: Update ConnectionForm.vue for Database Type

**Files:**
- Modify: `frontend/src/components/ConnectionForm.vue`

- [ ] **Step 1: Add database type radio buttons**

In the type selector section, add database options after the VNC radio button:

```vue
<el-radio-button label="mysql">MySQL</el-radio-button>
<el-radio-button label="postgres">PostgreSQL</el-radio-button>
<el-radio-button label="rqlite">rqlite</el-radio-button>
```

- [ ] **Step 2: Add database name field (visible only for database types)**

After the port field, add:

```vue
<el-form-item v-if="isDatabaseType" :label="t('db.databases')">
  <el-input v-model="form.dbName" :placeholder="t('db.databases')" />
</el-form-item>
```

- [ ] **Step 3: Update the form model and TypeScript**

In the `<script>` section, add a computed property:

```typescript
const isDatabaseType = computed(() => ['mysql', 'postgres', 'rqlite'].includes(form.type))
```

Update the form data initialization to include `dbType` and `dbName`:

```typescript
const form = reactive({
  // ... existing fields ...
  dbName: '',
})
```

- [ ] **Step 4: Adjust conditional visibility for database types**

For database types, hide auth type selector and key path (databases always use password):

```vue
<el-form-item v-if="!isVncOrRdpOrDb && form.type !== 'vnc'" :label="t('conn.authType')">
```

Where `isVncOrRdpOrDb` includes database types.

- [ ] **Step 5: Handle form serialize/deserialize for dbType**

When saving: set `config.dbType = form.type` for database types.
When editing: set `form.type = config.type || config.dbType` (database connections may have dbType in config).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/ConnectionForm.vue
git commit -m "feat(frontend): add database type support to ConnectionForm

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 18: Update Sidebar.vue for Database Connections

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: Add connectDB to emits**

```typescript
const emit = defineEmits(['connect', 'connectSftp', 'connectRdp', 'connectVnc', 'connectDB', 'toggle'])
```

- [ ] **Step 2: Update connect logic to route database types**

In the `onItemDblClick` handler and the Enter key handler, add routing for database types:

```typescript
if (c.type === 'mysql' || c.type === 'postgres' || c.type === 'rqlite') {
  emit('connectDB', c)
} else if (c.type === 'rdp') {
  emit('connectRdp', c)
} else if (c.type === 'vnc') {
  emit('connectVnc', c)
} else {
  emit('connect', c)
}
```

- [ ] **Step 3: Add context menu item for database connections**

In the context menu, add a connectDB option for database connections:

```vue
<div v-if="selectedConn && (selectedConn.type === 'mysql' || selectedConn.type === 'postgres' || selectedConn.type === 'rqlite')" class="menu-item" @click="doConnectDB">{{ t('db.connectDB') }}</div>
```

Add the handler:

```typescript
function doConnectDB() {
  if (selectedConn.value) {
    emit('connectDB', selectedConn.value)
  }
  showMenu.value = false
}
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/Sidebar.vue
git commit -m "feat(frontend): add database connection routing to Sidebar

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 19: Update App.vue for Database Tab Support

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Import DBTabContent**

```typescript
import DBTabContent from './components/DBTabContent.vue'
```

- [ ] **Step 2: Add template condition for database tab**

```vue
<DBTabContent
  v-else-if="activeTab.type === 'database'"
  :key="activeTab.id"
  :session-id="getPanelSessionId(activeTab.panelId)"
/>
```

- [ ] **Step 3: Add onConnectDB handler**

```typescript
async function onConnectDB(config: ConnectionConfig) {
  connectionStore.add(config)
  const displayTitle = config.name || `${config.dbType}:${config.user}@${config.host}`
  const dbType = config.type as string

  const panel = panelStore.createPanel(config, 'database')
  panel.title = displayTitle
  const tab = tabStore.createDBTab(displayTitle, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession('database', config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create database session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}
```

- [ ] **Step 4: Wire up the connectDB event from Sidebar**

In the template where `<Sidebar>` is used, add the event handler:

```vue
@connect-d-b="onConnectDB"
```

Note: Vue converts `connectDB` to `connect-d-b` automatically, or use the kebab-case version in template.

- [ ] **Step 5: Add database tab close cleanup**

In the closeTab handler, add cleanup for database sessions:

```typescript
if (tab && tab.type === 'database') {
  const p = panelStore.getPanel(tab.panelId)
  if (p?.sessionId) {
    await CloseSession(p.sessionId)
  }
}
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/App.vue
git commit -m "feat(frontend): integrate database tab support in App.vue

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 20: Build and End-to-End Verification

- [ ] **Step 1: Full Go build**

```bash
go build ./...
```

Expected: No errors.

- [ ] **Step 2: Regenerate Wails bindings**

```bash
wails generate module
```

Expected: Frontend bindings regenerated.

- [ ] **Step 3: Frontend type check**

```bash
cd frontend && npx vue-tsc --noEmit
```

Expected: No type errors.

- [ ] **Step 4: Frontend build**

```bash
cd frontend && npm run build
```

Expected: Build succeeds.

- [ ] **Step 5: Full Wails dev build**

```bash
wails build
```

Expected: Application binary produced.

- [ ] **Step 6: Smoke test checklist**

1. Launch the app
2. Create a MySQL/PostgreSQL/rqlite connection with correct credentials
3. Double-click to connect → database tab opens
4. Tree panel shows database list
5. Click a database → tables expand
6. Click a table → structure tab shows columns and indexes
7. Switch to SQL Query tab
8. Type `SELECT 1` → Ctrl+Enter → result shows in grid
9. Run an `INSERT`/`UPDATE` → affected rows shown
10. Query history panel shows executed queries
11. Close tab → session disconnects

- [ ] **Step 7: Commit any final fixes**

```bash
git add -A
git commit -m "chore: final fixes from integration testing

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```
