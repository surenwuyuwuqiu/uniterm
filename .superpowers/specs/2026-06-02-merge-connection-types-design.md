# Merge Connection Types Design

## Overview

合并新建/编辑连接界面中 SSH、Telnet、Mosh 三种远程终端协议为一个"终端"大类，与"远程桌面"（RDP/VNC）、"数据库"（MySQL/PostgreSQL/rqlite）并列。纯前端改动，后端接口和存储数据不受影响。

## Motivation

- SSH、Telnet、Mosh 本质都是远程终端连接，共享 host、port、user、password、postLoginScript 等字段
- 当前 7 个 radio-button 平铺在类型选择区，视觉杂乱
- 参照数据库的两层结构，统一交互模式

## Design

### UI Structure

```
┌─ 类型 ───────────────────────────────────────┐
│ [终端]    [远程桌面]    [数据库]              │  ← 第一层: category
├──────────────────────────────────────────────┤
│ [SSH]     [Telnet]      [Mosh]               │  ← 第二层: protocol (radio-button toggle)
└──────────────────────────────────────────────┘
```

选择"远程桌面"时第二层变为 `[RDP] [VNC]`（RDP 仅 Windows 显示），选择"数据库"时变为 `[MySQL] [PostgreSQL] [rqlite]`。

### Data Model (No Changes)

`form.type` 保持原值：`'ssh' | 'telnet' | 'mosh' | 'rdp' | 'vnc' | 'database'`

`category` 为纯前端计算值，不持久化：

```typescript
function typeToCategory(type: string): string {
  if (['ssh', 'telnet', 'mosh'].includes(type)) return 'terminal'
  if (['rdp', 'vnc'].includes(type)) return 'remote'
  if (type === 'database') return 'database'
  return 'terminal'
}

function categoryDefaultType(category: string): string {
  switch (category) {
    case 'terminal': return 'ssh'
    case 'remote': return 'rdp'
    case 'database': return 'mysql'
    default: return 'ssh'
  }
}
```

### Field Visibility Matrix

| Field | SSH | Telnet | Mosh | RDP | VNC | DB |
|-------|-----|--------|------|-----|-----|-----|
| name | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| host | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| port | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| user | ✓ | ✓ | ✓ | — | — | * |
| authType | ✓ | — | ✓ | — | — | — |
| password | ✓ | ✓ | ✓ | ✓ | ✓ | * |
| keyPath | key only | — | key only | — | — | — |
| postLoginScript | ✓ | ✓ | ✓ | — | — | — |
| RDP resolution | — | — | — | ✓ | — | — |
| dbName | — | — | — | — | — | * |

*数据库: user/password/dbName 在 rqlite 时隐藏

### Interaction Logic

1. 切换 category → `form.type` 自动设为该分类第一个协议，`form.port` 更新为默认端口
2. 编辑已有连接时，根据 `form.type` 反算 `category` 并回显
3. 数据库子类型 `form.dbType` 保持不变

### Files Changed

| File | Change |
|------|--------|
| `frontend/src/components/ConnectionForm.vue` | 重构类型选择为两层 radio-group；调整字段 v-if 条件 |
| `frontend/src/i18n/index.ts` | 添加 `conn.categoryTerminal`、`conn.categoryRemote`、`conn.categoryDatabase` |

### Backward Compatibility

- `form.type` 值不变：`'ssh'`, `'telnet'`, `'mosh'` 等
- `ConnectionConfig` 接口不变
- 已有连接数据无影响
- 后端 session manager 路由无变化
