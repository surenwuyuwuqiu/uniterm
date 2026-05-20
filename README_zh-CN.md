<div align="center">
  <img src="build/appicon.png" alt="uniTerm" width="128" height="128" />
  <h1>uniTerm</h1>
  <p>一款现代化跨平台终端模拟器，内置 AI 助理功能。</p>
</div>

[English](README.md)

## 功能特性

- **SSH 客户端** — 通过密码或私钥连接远程服务器，支持多标签页管理终端会话。提供 5 种配色方案、6 种等宽字体、可调节字号与历史行数，选中行为和右键功能均可配置。
- **SFTP 文件管理器** — 双栏并排浏览本地与远程文件，支持上传、下载、拖拽、删除、重命名等操作，传输任务按标签页独立跟踪，可暂停、继续或取消。
- **AI 助理** — 侧边栏对话，兼容 Anthropic 协议的 LLM，直接在终端中执行 Shell 命令。提供全部确认、仅高危确认、免确认三种执行模式，支持多轮对话记录持久化。
- **工作区与分屏** — 将多个终端标签页合并为工作区，支持水平或垂直分屏布局，拖拽面板边缘或标题栏即可自由调整大小和位置。
- **连接管理器** — 保存、搜索、编辑、复制服务器连接，支持分组管理和拖拽排序，可多选或范围选择进行批量连接、批量删除等操作。
- **主题与国际化** — 暗色、深蓝、浅色三种界面主题，支持跟随系统自动切换。简体中文与 English 双语界面。
- **跨平台** — 基于 Wails v2 构建，支持 Windows、macOS、Linux 三大桌面平台。

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
│   ├── session/                  # SSH/SFTP 会话管理
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
