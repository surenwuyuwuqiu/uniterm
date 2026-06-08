# 连接类型筛选器设计文档

## 背景

在连接列表的搜索栏旁增加一个连接类型筛选器，方便用户按类型快速过滤连接。

## 需求

- 在搜索栏右侧添加一个连接类型筛选按钮
- 默认选中 ALL，显示所有连接
- 下拉菜单列出当前连接列表中存在的连接类型
- `database` 类型按 `dbType` 细分为 MySQL、PostgreSQL、rqlite
- 类型筛选与文本搜索同时生效（AND 逻辑）
- 不显示每种类型的数量

## 连接类型定义

根据 `ConnectionConfig.type` 和 `ConnectionConfig.dbType`：

| 显示名称 | 匹配规则 |
|---------|---------|
| SSH | `type === 'ssh'` |
| Telnet | `type === 'telnet'` |
| Mosh | `type === 'mosh'` |
| RDP | `type === 'rdp'` |
| VNC | `type === 'vnc'` |
| SPICE | `type === 'spice'` |
| MySQL | `type === 'database' && dbType === 'mysql'` |
| PostgreSQL | `type === 'database' && dbType === 'postgres'` |
| rqlite | `type === 'database' && dbType === 'rqlite'` |
| Local | `type === 'local'` |
| SFTP | `type === 'sftp'` |
| Monitor | `type === 'monitor'` |

## 方案

采用 **方案 B：搜索框内嵌 suffix 按钮**

### UI 布局

在 `Sidebar.vue` 的 `el-input` 组件的 `suffix` slot 内放置一个筛选图标按钮：

- 使用 `Filter` 图标（来自 `@lucide/vue`）
- 默认状态：颜色为 `var(--text-muted)`
- 激活状态（选中非 ALL 类型）：颜色变为 `var(--accent)`

### 下拉菜单

点击筛选图标后，在图标下方弹出下拉菜单：

- 第一项：`ALL`（默认选中，带 ✓ 标记）
- 分隔线
- 后续项：从当前 `connectionStore.connections` 动态提取的连接类型列表（去重、按字母排序）
- 不显示类型数量

### 过滤逻辑

修改 `filteredGrouped` computed，同时应用文本搜索和类型筛选：

1. 文本搜索：与现有逻辑一致（匹配 `name` 或 `host`）
2. 类型筛选：
   - 普通类型：`conn.type === selectedType`
   - database 子类型：`conn.type === 'database' && conn.dbType === dbSubType`
   - 组合键格式：`"database:mysql"`，解析时按 `:` 分割

两个条件同时满足时才显示（AND 逻辑）。

### 状态管理

- `selectedTypeFilter`：ref，默认 `'all'`
- 筛选状态仅保存在内存中，页面刷新后恢复为 ALL

### 键盘导航

保持现有上下箭头、Enter 键行为不变。筛选后的结果列表同样支持键盘导航。

## 影响文件

- `frontend/src/components/Sidebar.vue`

## 待添加的 i18n 键

- `sidebar.filterAll` — "全部"
