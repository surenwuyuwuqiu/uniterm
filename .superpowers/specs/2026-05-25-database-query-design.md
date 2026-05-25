# 数据库查询功能设计

## 概述

为 uniTerm 添加数据库客户端功能，支持 MySQL、PostgreSQL 和 rqlite。用户可以在连接管理器中创建数据库连接（与 SSH/RDP 并列），通过树形视图浏览数据库和表，查看/编辑表结构，执行 SQL 查询并查看结果表格，以及访问按连接保存的查询历史。

## 数据库支持

| 数据库      | 驱动                          | 说明                                      |
|------------|-------------------------------|-------------------------------------------|
| MySQL      | go-sql-driver/mysql           | TCP 直连                                   |
| PostgreSQL | lib/pq                        | TCP 直连                                   |
| rqlite     | gorqlite/stdlib               | HTTP API，通过 gorqlite 提供 `database/sql` 驱动 |

三种数据库统一走 xorm 作为 ORM 层 —— xorm 兼容任意 `database/sql` 驱动。

rqlite 通过 xorm 的说明：gorqlite 提供了标准 `database/sql` 驱动（`github.com/rqlite/gorqlite/stdlib`），xorm 可以像打开其他数据库一样打开 rqlite。rqlite 的事务/预编译语句限制（均为 no-op）不影响本功能，因为我们只用 xorm 的 `Query()`/`Exec()` 和 `DBMetas()` 表结构自省。

## 新增依赖

| 包名 | 用途 | 预估体积 |
|------|------|---------|
| `xorm.io/xorm` | 统一 ORM、表结构自省、查询执行 | ~2-3 MB |
| `github.com/go-sql-driver/mysql` | MySQL 驱动 | ~1.5 MB |
| `github.com/lib/pq` | PostgreSQL 驱动 | ~1.5 MB |
| `github.com/rqlite/gorqlite` | rqlite `database/sql` 驱动 | ~200 KB |
| **合计** | | **~5-6 MB** |

## 连接配置

数据库连接在现有 `ConnectionConfig` 基础上扩展字段：

```go
type ConnectionConfig struct {
    // ... 现有字段 ...

    // 数据库专用字段
    DBType     string `json:"dbType,omitempty"`     // "mysql"、"postgres"、"rqlite"
    DBName     string `json:"dbName,omitempty"`     // 默认数据库名
}
```

- 主机/端口/用户名/密码复用现有字段
- 密码复用现有 `PasswordStore`（OS 密钥链）机制
- v1 不做 SSH 隧道

## 架构

### 后端包结构

```
backend/
  database/
    engine.go          // xorm.Engine 封装，统一处理 MySQL/PG/rqlite
    schema.go          // 表结构自省，基于 engine.DBMetas()
    executor.go        // SQL 执行 + 结果集序列化
    history.go         // 按连接保存查询历史
```

不需要每种驱动单独写文件 —— xorm 统一处理驱动差异。

### DatabaseSession

`DatabaseSession` 实现现有 `Session` 接口：

- `Write(data)` — 执行前端传来的 SQL
- `SetOnDataCallback` — 将结构化结果（列定义 + 行数据）以 JSON 推送给前端
- 由 `SessionManager` 管理生命周期（与 SSH/SFTP 相同）
- Session 类型：`"database"`

```go
type DatabaseSession struct {
    baseSession
    engine *xorm.Engine  // 统一的 xorm 引擎，适配 mysql/postgres/rqlite
    dbType string        // "mysql"、"postgres"、"rqlite"
}
```

`SessionManager.Create("database", config)` 创建 session，内部根据 `config.DBType` 打开对应的 `xorm.Engine`。

### 用到的 xorm API

| 功能 | xorm API |
|------|----------|
| 打开连接 | `xorm.NewEngine(driverName, dsn)` |
| 列出数据库 | `engine.Query("SHOW DATABASES")`（MySQL）/ `engine.Query("SELECT datname FROM pg_database")`（PG） |
| 列出表 | `engine.DBMetas()` |
| 列信息 | `engine.DBMetas()` 返回 `[]*schemas.Table`，含列名、类型、是否可空等 |
| 索引信息 | `engine.DBMetas()` 返回每张表的索引信息 |
| 执行 SELECT | `engine.Query(sql)` |
| 执行 DML/DDL | `engine.Exec(sql)` |
| 修改表结构 | `engine.Exec("ALTER TABLE ...")` |

### Wails 绑定方法

暴露在 `App` 上：

```
// 连接发现
GetDatabases(sessionID string) -> []string
GetTables(sessionID, dbName string) -> []TableInfo
GetTableSchema(sessionID, dbName, tableName string) -> SchemaResult

// SQL 执行
ExecuteQuery(sessionID, sql string) -> QueryResult    // SELECT → {columns, rows}
ExecuteStatement(sessionID, sql string) -> ExecResult  // INSERT/UPDATE/DELETE/DDL → {affected, lastInsertId}

// 表结构修改
AlterTable(sessionID, dbName, tableName string, changes SchemaChanges) -> error

// 查询历史
GetQueryHistory(sessionID string) -> []HistoryEntry
ClearQueryHistory(sessionID string) -> error
```

## 前端

### 新增文件

```
frontend/src/
  components/
    DBTabContent.vue        // 数据库标签页主布局（左树 + 右面板）
    DBTreePanel.vue         // 左侧：database → tables 树形列表
    DBTableStructure.vue    // 右侧 Tab 1：表列信息 + 索引查看/编辑
    DBQueryEditor.vue       // 右侧 Tab 2：SQL 编辑器 + 结果表格
    DBQueryHistory.vue      // 底部：查询历史列表
  types/
    database.ts             // 数据库相关 TypeScript 类型
```

### UI 布局

```
┌──────────────────────────────────────────────────┐
│  [DB: mysql@localhost]  Tab Bar                   │
├──────────┬───────────────────────────────────────┤
│ 树形列表  │  [表结构] [SQL 查询]                   │
│          ├───────────────────────────────────────┤
│  db1     │  Columns:                             │
│   table1 │  ┌──────┬──────┬───────┬──────┐      │
│   table2 │  │ 列名  │ 类型  │ 可空  │ 默认值 │     │
│  db2     │  ├──────┼──────┼───────┼──────┤      │
│          │  │ id   │ INT  │ NO    │ -     │      │
│          │  │ name │ TEXT │ YES   │ NULL  │      │
│          │  └──────┴──────┴───────┴──────┘      │
│          │  Indexes: ...                         │
│          ├───────────────────────────────────────┤
│          │  查询历史                              │
│          │  ┌──────────────────────────────┐     │
│          │  │ SELECT * FROM users LIMIT 10 │     │
│          │  │ 2026-05-25 14:30             │     │
│          │  └──────────────────────────────┘     │
└──────────┴───────────────────────────────────────┘
```

- **左侧面板**：可调整宽度的树形视图，展示 数据库 → 表 的层级结构
- **右上**：Tab 切换 — "表结构" 和 "SQL 查询"
  - 表结构 Tab：点击树中的表后显示，展示列信息和索引信息，支持内联编辑
  - SQL 查询 Tab：始终可用，含 SQL 编辑器 + 结果表格
- **右下**：按连接保存的查询历史，点击可回填到编辑器中

### Tab 类型扩展

```typescript
// workspace.ts
export type Tab = TerminalTab | SettingsTab | WorkspaceTab | SFTPTab | RDPTab | VNCTab | DBTab
export type PanelType = 'ssh' | 'sftp' | 'settings' | 'rdp' | 'vnc' | 'local' | 'database' | 'other'

export interface DBTab {
  type: 'database'
  id: string
  panelId: string
  name: string
}
```

### 交互流程

1. 用户在连接管理器中新建数据库连接（类型选择 MySQL/PostgreSQL/rqlite）
2. 点击连接 → 打开 `database` 类型标签页，加载 `DBTabContent`
3. 后端创建 `DatabaseSession`，打开 xorm 引擎，推送数据库列表
4. 用户在树中点击数据库 → 展开表列表
5. 用户点击某张表 → 打开表结构 Tab，显示列和索引信息
6. 用户切换到 SQL 查询 Tab → 编写 SQL，Ctrl+Enter 执行
7. 结果在编辑器下方的表格中展示
8. 执行的查询自动保存到该连接的查询历史

## 密码存储

数据库密码复用现有 `PasswordStore` 接口（OS 密钥链）：

- `ConnectionStore.PasswordStore` 已有 `GetPassword`/`SetPassword`/`DeletePassword` 方法
- `authType == "password"` 时，密码存储在 OS 密钥链中
- 数据库连接使用相同的 `authType: "password"` 字段
- `ConnectionStore.Save()` 写入 JSON 前将密码提取到密钥链
- `ConnectionStore.populatePasswords()` 从密钥链加载密码到内存

无需新增任何密码处理逻辑。

## 查询历史存储

```go
type HistoryEntry struct {
    ID         string    `json:"id"`
    SQL        string    `json:"sql"`
    ExecutedAt time.Time `json:"executedAt"`
    Duration   int64     `json:"durationMs"`
    Error      string    `json:"error,omitempty"`
    RowCount   int       `json:"rowCount,omitempty"`
}
```

- 按连接存储在 JSON 文件中：`<data-dir>/db_history/<connection-id>.json`
- 每个连接最多保留 500 条，超出后淘汰最旧的
- 前端通过 Wails 绑定获取，在历史面板中展示
- 点击某条历史记录，SQL 回填到编辑器中

## 实施阶段

### 阶段 1：核心连接
- [ ] `database/engine.go` — xorm engine 封装（通过 xorm 打开 MySQL/PG/rqlite）
- [ ] `session/database_session.go` — DatabaseSession 实现 Session 接口
- [ ] `ConnectionConfig` — 新增 `DBType`、`DBName` 字段
- [ ] `SessionManager` — 接入 `"database"` session 创建

### 阶段 2：Schema 浏览器
- [ ] `database/schema.go` — 通过 `engine.DBMetas()` 实现表结构自省
- [ ] Wails 绑定：`GetDatabases`、`GetTables`、`GetTableSchema`
- [ ] `DBTabContent.vue` — 主布局（可拖拽分割面板）
- [ ] `DBTreePanel.vue` — 数据库/表树形列表
- [ ] `DBTableStructure.vue` — 只读模式列信息 + 索引查看

### 阶段 3：SQL 编辑器 + 查询执行
- [ ] `database/executor.go` — `engine.Query()` / `engine.Exec()` + 结果序列化
- [ ] Wails 绑定：`ExecuteQuery`、`ExecuteStatement`
- [ ] `DBQueryEditor.vue` — SQL 编辑器 + 结果表格 + 行内编辑
  - 双击结果表格的单元格进入编辑模式
  - 按 Enter 确认 → 自动生成 `UPDATE table SET col = value WHERE pk = pkValue` 并执行
  - 需要前端解析 SQL 中的表名和主键列（从 GetTableSchema 获取）

### 阶段 4：表结构编辑
- [ ] 通过 `engine.Exec("ALTER TABLE ...")` 实现 DDL
- [ ] Wails 绑定：`AlterTable`
- [ ] `DBTableStructure.vue` 内联编辑功能

### 阶段 5：查询历史
- [ ] `database/history.go` — 按连接的查询历史持久化
- [ ] Wails 绑定：`GetQueryHistory`、`ClearQueryHistory`
- [ ] `DBQueryHistory.vue` — 历史面板，点击回填 SQL

### 阶段 6：前端集成
- [ ] `workspace.ts` — 新增 `DBTab` 类型、`PanelType` 扩展
- [ ] `tabStore.ts` — 新增 `createDBTab`
- [ ] ConnectionForm — 新增数据库类型选择和专用字段
- [ ] Sidebar — 连接列表中展示数据库连接
- [ ] i18n — 所有新 UI 的中英文文案

## v1 不做

- SSH 隧道连接数据库
- SSL/TLS 证书配置
- 一个连接内多个 SQL 编辑器标签页
- 数据导出（CSV、JSON）
- 存储过程 / 函数 / 视图管理
- 连接池配置
