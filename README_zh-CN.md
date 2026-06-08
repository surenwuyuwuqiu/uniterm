<div align="center">
  <img src="build/appicon.png" alt="uniTerm" width="128" height="128" />
  <h1>uniTerm</h1>
  <p>一款现代化跨平台终端模拟器，内置可自主执行的 AI Agent —— 能够像 Claude Code 一样独立规划并执行多轮 Shell 命令。</p>
</div>

[English](README.md)

## 功能特性

### AI 助理

自主执行的 AI Agent，像 Claude Code 一样独立规划并执行多轮 Shell 命令，直接在终端中完成复杂任务。

- **自主多轮执行** — AI Agent 能够自主规划、执行、观察结果并迭代，在多轮 Shell 命令中无需人工干预即可完成复杂操作。
- **大模型集成** — 侧边栏对话，兼容 Anthropic 协议，支持 Claude 及其他兼容模型。
- **灵活的执行模式** — 提供免确认、仅高危确认、写操作确认、全部确认四种模式，自主权由你掌控。
- **对话持久化** — 会话聊天记录按标签页保存，重新打开应用后历史记录仍然保留。
- **终端智能协作** — AI 命令直接在当前终端标签页中执行，支持固定到指定标签页或跟随当前激活终端。分屏中人与 AI 各司其职，同屏协作互不干扰。
- **智能补全** — SSH 终端输入时，根据历史命令和 AI 能力实时提供命令补全建议。

### 全功能终端

- **SSH / Telnet / Mosh** — 支持密码或私钥认证连接远程服务器。支持 SSH、Telnet、Mosh（低延迟移动网络连接）。多标签页管理，提供 5 种配色方案、6 种等宽字体、可调节字号与历史行数，选中行为和右键功能均可配置。
- **SFTP 文件管理器** — 双栏并排浏览本地与远程文件。支持上传、下载、拖拽、删除、重命名等操作，传输任务按标签页独立跟踪，可暂停、继续或取消。
- **服务器监控** — 实时监看已连接服务器的运行状态。支持性能指标（CPU、内存、磁盘、网络）、进程列表及详情、监听端口、磁盘用量与挂载信息、网卡列表及 bond/bridge 识别。
- **本地终端** — 支持 PowerShell、CMD、Git Bash 等常见 Shell 的本地终端，与 SSH 会话共享字体、配色和操作设置。
- **RDP / VNC / SPICE** — 支持 Windows 远程桌面、VNC 和 SPICE 连接。
- **数据库客户端** — 连接 MySQL、PostgreSQL、rqlite 数据库，支持 SQL 查询执行、表结构浏览、数据行在线编辑，统一界面管理全部数据源。

### 自定义能力

- **连接管理器** — 保存、搜索、编辑、分组、复制服务器连接，支持拖拽排序，可多选或范围选择进行批量连接、批量删除等操作。
- **工作区与自由分屏** — 将多个终端标签页合并为工作区，支持水平或垂直分屏布局，拖拽面板边缘或标题栏即可自由调整大小和位置。
- **云端同步** — 基于 GitHub、GitLab、Gitee 私有仓库构建专属私人云同步仓库，所有配置（连接信息、AI 模型密钥、应用设置）经 AES-256-GCM 加密后保存至远端。支持自动同步、冲突解决、主密码修改和仓库绑定管理。
- **主题** — 暗色、深蓝、浅色三种界面主题，支持跟随系统自动切换。
- **国际化** — 简体中文与 English 双语界面，基于清晰的 i18n 架构，方便扩展更多语言。
- **跨平台** — 基于 Wails v2 构建，原生运行于 Windows、macOS、Linux 三大桌面平台。

## 使用流程

### SSH 连接

1. 在连接管理器中点击**新建连接**
2. 填入主机、端口和认证信息（密码或私钥）
3. 点击**连接**打开 SSH 终端会话

### AI 助理

1. 进入设置页面，配置你的 **AI 大模型**（API 地址、模型和密钥）
2. 打开一个终端标签页（SSH 或本地）
3. 打开 AI 侧边栏对话，输入需求 —— AI Agent 直接在终端中执行命令

### SFTP 文件传输

1. 在连接管理器中**右键**一个 SSH 连接
2. 选择**连接 SFTP**
3. 在双栏文件管理器中浏览、上传、下载或拖拽文件

## 技术栈

| 层级 | 技术 |
|------|------|
| 桌面框架 | Wails v2 |
| 后端 | Go |
| 前端 | Vue 3 + Pinia + Element Plus |
| 终端引擎 | xterm.js |
| AI 协议 | Anthropic Messages API |

## 环境要求

- [Go](https://go.dev/dl/) 1.23+
- [Node.js](https://nodejs.org/) 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2

### 平台依赖

- **Windows**: WebView2 运行时（Windows 10+ 已内置）
- **macOS**: Xcode Command Line Tools
- **Linux**: `libgtk-3-dev` 和 `libwebkit2gtk-4.1-dev`

## 快速开始

```bash
git clone https://github.com/ys-ll/uniterm.git
cd uniTerm
cd frontend && npm install && cd ..
wails dev                   # 开发模式运行
wails build                 # 构建生产版本
```

## 项目结构

```
uniTerm/
├── main.go                       # 入口文件
├── app.go                        # Wails 绑定、LLM API 代理、SFTP API
├── backend/
│   ├── session/                  # SSH/SFTP/数据库 会话管理
│   ├── database/                 # SQL 执行、表结构查询、DSN 构建
│   ├── store/                    # 持久化配置（连接、AI、设置）
│   └── log/                      # 文件日志
├── frontend/
│   └── src/
│       ├── components/           # Vue 组件
│       ├── composables/          # 终端组合式函数
│       ├── stores/               # Pinia 状态管理
│       ├── services/             # AI 代理循环、LLM 客户端
│       ├── i18n/                 # 国际化翻译
│       └── types/                # TypeScript 类型定义
└── wails.json
```

## 开源协议

Apache 2.0
