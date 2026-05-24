# 配置云端同步 — 设计文档

## 概述

基于 Git 私有仓库（GitHub/Gitee）实现 uniTerm 配置的云端同步。用户将配置文件版本化到自有私有仓库，通过内嵌 go-git 库实现拉取/推送，不依赖系统安装的 git。

## 同步范围

| 文件 | 内容 | 加密 |
|------|------|------|
| `connections.json` | SSH/RDP/VNC 连接配置 | password 字段加密 |
| `ai-config.json` | AI 接口配置 | apiKey 字段加密 |

`settings.json` 和 `ai-sessions.json` 不参与同步。

## 同步触发

### 立即同步（手动触发）

用户在设置页点击"立即同步"按钮：
- 先 commit 当前本地变更（若工作区有改动）
- fetch 远端，比较两个分支
- 本地有新提交、远端无 → 自动 push
- 远端有新提交、本地无 → 自动 pull
- 两边都有新提交 → 弹出冲突方向选择对话框
- 两边相同 → 提示"已是最新"

### 自动同步

设置页提供开关，开启后：
- 配置修改保存时：commit + push（后台静默，无冲突时不弹窗）
- 应用启动时：fetch + pull（若有冲突则弹窗）

后台静默推送时若无冲突直接完成；有冲突则弹出方向选择对话框。用户取消则跳过本次同步，下次继续提示。

## 冲突处理

任何时候本地和远端都有新提交时（分叉），弹出方向选择对话框：

- "用本地覆盖远端" → push --force
- "用远端覆盖本地" → fetch + reset --hard
- "取消" → 跳过本次同步

对话框显示双方最后修改时间供参考。Git 历史保留所有版本，旧数据可随时回溯。

## 仓库认证

支持两种方式：
- **SSH Key**：复用系统已有 SSH Key（`~/.ssh/`）
- **HTTPS + Personal Access Token**：用户在设置页填入 Token

## Git 实现

使用 `go-git` 纯 Go 库，不依赖系统安装 git。

## 敏感数据保护

### OS 密钥链

使用 `github.com/zalando/go-keyring` 跨平台库：
- Windows：凭据管理器
- macOS：Keychain
- Linux：Secret Service / D-Bus

存储内容：
- `uniTerm/encryption-key`：32 字节随机 AES-256 密钥
- `uniTerm/conn/<connection-id>`：连接密码
- `uniTerm/git-token`：Git PAT（仅 token 认证模式）

### 加密密钥

首次使用同步或首次存储连接密码时生成随机 32 字节密钥，存入 OS 密钥链，应用启动时自动读取到内存。用户完全无感。

### 字段加密

推送前加密、拉取后解密。本地 JSON 文件保持明文。

**算法：** AES-256-GCM
- 随机 12 字节 nonce
- 密文 + nonce → Base64 编码

**加密字段：**
- `connections[].password`
- `ai-config.apiKey`

## 连接密码迁移

应用启动时执行一次存量迁移：

1. 加载 `connections.json`
2. 遍历每个连接，若 `authType == "password"` 且 `password` 非空：
   - 写入密钥链 `uniTerm/conn/<id>`
   - `password` 字段置空
3. 保存 `connections.json`

后续新增/修改连接时直接写密钥链。加载时从密钥链读取填充到内存。删除连接时清理密钥链对应条目。

密钥链不可用时回退读取 JSON 中 `password` 字段。迁移失败不阻塞主流程。

## 模块架构

```
backend/
  sync/
    git.go           -- go-git 封装 (clone/pull/push/status)
    keychain.go      -- OS 密钥链封装
    crypto.go        -- AES-256-GCM 加密/解密
    sync_service.go  -- 同步编排 (手动/自动同步流程)
    sync_config.go   -- 本地同步配置持久化
```

远程仓库目录结构：
```
<repo>/
  connections.json    -- 加密后的连接配置
  ai-config.json      -- 加密后的 AI 配置
```

本地同步配置文件 `sync-config.json`（存于 `os.UserConfigDir()/uniTerm/`，不参与同步）：

```json
{
  "repoUrl": "https://github.com/user/config-repo.git",
  "branch": "main",
  "authType": "ssh",
  "autoSync": true,
  "lastSyncAt": "2026-05-24T14:30:00Z"
}
```

PAT 通过 OS 密钥链存储（`uniTerm/git-token`），不在 sync-config.json 中落盘。

## UI 设计

设置页新增"配置同步"标签页：

- **Git 仓库地址**：文本输入框，支持 HTTPS 和 SSH 格式
- **认证方式**：Radio 切换 SSH Key / PAT
- **Token**：密码输入框（仅 PAT 模式）
- **自动同步**：Switch 开关
- **上次同步时间**：只读文本
- **测试连接**：按钮，调用 `git ls-remote` 验证
- **立即同步**：按钮，自动判断方向

## 数据流

```
前端设置页 → Wails Bind → sync_service.go
                                ├── git.go (go-git)
                                ├── crypto.go (加解密)
                                └── keychain.go (密钥)
                                │
                              本地文件 ↔ 远程 Git 仓库
```

## 前端 Pinia Store

新增 `syncStore.ts`：
- `syncConfig`：同步配置状态
- `loadConfig()`：加载本地同步配置
- `saveConfig(config)`：保存同步配置
- `sync()`：调用后端立即同步
- `testConnection()`：测试仓库连接

## 排期说明

- 加密密钥导出/导入机制（换电脑恢复）本期不做，预留入口
- 连接导入/导出功能后续扩展
