# Database Query Feature Design

## Overview

Add database client functionality to uniTerm, supporting MySQL, PostgreSQL, SQLite, and rqlite. Users can create database connections alongside existing SSH/RDP connections, browse databases/tables in a tree view, view/edit table schemas, execute SQL queries with result grids, and access per-connection query history.

## Database Support

| Database   | Driver              | Notes                            |
|------------|---------------------|----------------------------------|
| MySQL      | go-sql-driver/mysql | TCP connection                   |
| PostgreSQL | lib/pq              | TCP connection                   |
| SQLite     | modernc.org/sqlite  | Pure Go, file-based, no CGO      |
| rqlite     | HTTP + sqlite3      | HTTP API, SQLite-compatible SQL  |

## Connection Configuration

Database connections extend `ConnectionConfig` with additional fields:

```go
type ConnectionConfig struct {
    // ... existing fields ...

    // Database-specific fields
    DBType     string `json:"dbType,omitempty"`     // "mysql", "postgres", "sqlite", "rqlite"
    DBName     string `json:"dbName,omitempty"`     // default database name
    DBPath     string `json:"dbPath,omitempty"`     // SQLite file path (local only)
}
```

- Host/Port/User/Password reuse existing fields
- Password reuse existing `PasswordStore` (OS keychain) mechanism
- No SSH tunneling in v1

## Architecture

### Backend Package Structure

```
backend/
  database/
    driver.go          // Driver interface
    mysql.go           // MySQL driver
    postgres.go        // PostgreSQL driver
    sqlite.go          // SQLite driver (pure Go, no CGO)
    rqlite.go          // rqlite driver (HTTP API)
    schema.go          // Schema introspection (columns, indexes)
    executor.go        // Query execution + result serialization
    history.go         // Per-connection query history persistence
```

### Driver Interface

```go
type Driver interface {
    Open(config ConnectionConfig) (*sql.DB, error)
    GetDatabases() ([]string, error)
    GetTables(dbName string) ([]TableInfo, error)
    GetColumns(dbName, tableName string) ([]ColumnInfo, error)
    GetIndexes(dbName, tableName string) ([]IndexInfo, error)
    AlterColumn(dbName, tableName string, col ColumnInfo) error
    AddColumn(dbName, tableName string, col ColumnInfo) error
    DropColumn(dbName, tableName string, colName string) error
    AddIndex(dbName, tableName string, idx IndexInfo) error
    DropIndex(dbName, tableName string, idxName string) error
}
```

### DatabaseSession

`DatabaseSession` implements the existing `Session` interface:

- `Write(data)` — execute SQL from frontend
- `SetOnDataCallback` — push structured results (columns + rows) as JSON
- Session lifecycle managed by `SessionManager` (same as SSH/SFTP)
- Connection type: `"database"` (or per-db-type: `"mysql"`, `"postgres"`, etc.)

### Wails Binding Methods

Exposed on `App`:

```
// Connection discovery
GetDatabases(sessionID string) -> []string
GetTables(sessionID, dbName string) -> []TableInfo
GetTableSchema(sessionID, dbName, tableName string) -> SchemaResult

// Query execution
ExecuteQuery(sessionID, sql string) -> QueryResult    // SELECT
ExecuteStatement(sessionID, sql string) -> ExecResult  // INSERT/UPDATE/DELETE/DDL

// Schema modification
AlterTable(sessionID, dbName, tableName string, changes SchemaChanges) -> error

// Query history
GetQueryHistory(sessionID string) -> []HistoryEntry
ClearQueryHistory(sessionID string) -> error
```

## Frontend

### New Files

```
frontend/src/
  components/
    DBTabContent.vue        // Main database tab layout (left tree + right panels)
    DBTreePanel.vue         // Left: database > tables tree
    DBTableStructure.vue    // Right tab 1: table columns + indexes view/edit
    DBQueryEditor.vue       // Right tab 2: SQL editor (textarea/CodeMirror) + result grid
    DBQueryHistory.vue      // Bottom: query history list per connection
  types/
    database.ts             // DB-specific TypeScript types
```

### UI Layout

```
┌──────────────────────────────────────────────────┐
│  [DB: mysql@localhost]  Tab Bar                   │
├──────────┬───────────────────────────────────────┤
│ Tree     │  [Table Structure] [SQL Query]        │
│          ├───────────────────────────────────────┤
│  db1     │  Columns:                             │
│   table1 │  ┌──────┬──────┬───────┬──────┐      │
│   table2 │  │ Name │ Type │ Null  │ Default│     │
│  db2     │  ├──────┼──────┼───────┼──────┤      │
│          │  │ id   │ INT  │ NO    │ -     │      │
│          │  │ name │ TEXT │ YES   │ NULL  │      │
│          │  └──────┴──────┴───────┴──────┘      │
│          │  Indexes: ...                         │
│          ├───────────────────────────────────────┤
│          │  Query History                        │
│          │  ┌──────────────────────────────┐     │
│          │  │ SELECT * FROM users LIMIT 10 │     │
│          │  │ 2026-05-25 14:30             │     │
│          │  └──────────────────────────────┘     │
└──────────┴───────────────────────────────────────┘
```

- **Left panel**: resizable tree view showing databases → tables
- **Right top**: tabbed panels — "Table Structure" and "SQL Query"
  - Table Structure tab: visible when a table is selected in the tree; shows columns and indexes with inline edit capability
  - SQL Query tab: always available; text editor + result grid
- **Right bottom**: query history panel showing past queries for this connection, click to replay

### Tab Types Extension

```typescript
// workspace.ts
export type Tab = TerminalTab | SettingsTab | WorkspaceTab | SFTPTab | RDPTab | VNCTab | DBTab
export type PanelType = 'ssh' | 'sftp' | 'settings' | 'rdp' | 'vnc' | 'local' | 'database' | 'other'

// New:
export interface DBTab {
  type: 'database'
  id: string
  panelId: string
  name: string
}
```

### Interaction Flow

1. User creates a database connection in Connection Manager (type: MySQL/PostgreSQL/SQLite/rqlite)
2. Click connect → opens a `database` type tab with `DBTabContent`
3. Backend creates `DatabaseSession`, connects, pushes database list
4. User clicks a database in the tree → expands table list
5. User clicks a table → opens Table Structure tab showing columns and indexes
6. User switches to SQL Query tab → writes SQL, executes (Ctrl+Enter)
7. Results render in a grid below the editor
8. Queries are saved to per-connection history automatically

## Password Storage

Database passwords reuse the existing `PasswordStore` interface (OS keychain):

- `ConnectionStore.PasswordStore` already handles `GetPassword`/`SetPassword`/`DeletePassword`
- When `authType == "password"`, the password is stored in OS keychain
- Database connections use the same `authType: "password"` field
- `ConnectionStore.Save()` extracts passwords to keychain before writing JSON
- `ConnectionStore.populatePasswords()` loads passwords from keychain into memory

No new password machinery needed.

## Query History Storage

```go
type HistoryEntry struct {
    ID        string    `json:"id"`
    SQL       string    `json:"sql"`
    ExecutedAt time.Time `json:"executedAt"`
    Duration  int64     `json:"durationMs"`
    Error     string    `json:"error,omitempty"`
    RowCount  int       `json:"rowCount,omitempty"`
}
```

- Stored per-connection in a JSON file: `<data-dir>/db_history/<connection-id>.json`
- Max 500 entries per connection, oldest evicted
- Frontend fetches via Wails binding, renders in history panel
- Click an entry to replay the SQL into the editor

## Implementation Phases

### Phase 1: Core Database Connection
- [ ] `database/driver.go` — Driver interface
- [ ] `database/mysql.go` — MySQL driver
- [ ] `database/postgres.go` — PostgreSQL driver
- [ ] `database/sqlite.go` — SQLite driver
- [ ] `session/database_session.go` — DatabaseSession implementing Session interface
- [ ] `ConnectionConfig` — add DB fields
- [ ] `SessionManager.Create("database", ...)` — wire up database session creation

### Phase 2: Schema Browser
- [ ] `database/schema.go` — introspection (columns, indexes)
- [ ] Wails bindings: `GetDatabases`, `GetTables`, `GetTableSchema`
- [ ] `DBTabContent.vue` — main layout with split panes
- [ ] `DBTreePanel.vue` — database/table tree
- [ ] `DBTableStructure.vue` — read-only columns + indexes view

### Phase 3: SQL Editor + Query Execution
- [ ] `database/executor.go` — query execution with safe SQL
- [ ] Wails bindings: `ExecuteQuery`, `ExecuteStatement`
- [ ] `DBQueryEditor.vue` — SQL editor + result grid

### Phase 4: Schema Editing
- [ ] DDL methods in drivers: `AlterColumn`, `AddColumn`, `DropColumn`, `AddIndex`, `DropIndex`
- [ ] Wails binding: `AlterTable`
- [ ] Inline editing in `DBTableStructure.vue`

### Phase 5: Query History
- [ ] `database/history.go` — per-connection history persistence
- [ ] Wails bindings: `GetQueryHistory`, `ClearQueryHistory`
- [ ] `DBQueryHistory.vue` — history panel with click-to-replay

### Phase 6: Frontend Integration
- [ ] `workspace.ts` — add `DBTab` type, `PanelType` extension
- [ ] `tabStore.ts` — add `createDBTab`
- [ ] ConnectionForm — add database type selection and fields
- [ ] Sidebar — show database connections in connection list
- [ ] i18n — Chinese/English strings for all new UI

## Out of Scope (v1)

- SSH tunneling for database connections
- SSL/TLS certificate configuration
- Multi-tab SQL editors within one connection
- Table data editor (inline row editing in result grid)
- Data export (CSV, JSON)
- Stored procedure / function / view management
- Connection pooling configuration
