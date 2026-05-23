# uniTerm Intro Website Design

## Overview

Single-page landing page for uniTerm open-source project, hosted via GitHub Pages from `docs/` directory. Pure static HTML + CSS, zero build dependencies.

## Audience & Goals

- **Audience**: Developers + ops/admin, both equally
- **Primary goal**: Drive downloads/installs and GitHub Stars
- **Hosting**: GitHub Pages, `docs/` folder on main branch

## Site Structure

Single page, five sections in vertical scroll order:

### 1. Hero

- Centered layout, dark background
- App icon (128px, current `build/appicon.png`)
- Title: "uniTerm"
- Subtitle: "一款现代化跨平台终端模拟器，内置 AI 助理功能"
- Two buttons, side by side:
  - **下载安装** — primary, filled blue
  - **GitHub** — secondary, outline style (links to `https://github.com/ys-ll/uniterm`)
- Responsive: stack buttons vertically on narrow screens

### 2. Features

- 2×3 card grid (single column on mobile)
- Each card: icon + title + one-sentence description
- Six cards:

| Icon | Title | Description |
|------|-------|-------------|
| Terminal | SSH 客户端 | 密码/私钥认证、多标签页管理、5 种配色、可定制字体与操作 |
| Folder | SFTP 文件管理 | 双栏浏览本地与远程文件，上传下载拖拽，传输任务按标签跟踪 |
| Robot/AI | AI 助理 | 侧边对话、终端执行命令，三种确认模式，多轮对话持久化 |
| Split layout | 工作区分屏 | 标签合并为工作区，水平/垂直分屏，拖拽自由调整布局 |
| Server | 连接管理 | 分组、搜索、批量操作，多选/范围选择连接 |
| Palette | 主题国际化 | 暗色/深蓝/浅色主题，跟随系统切换，中英双语 |

### 3. Screenshots

- Full-window app screenshot as main visual
- Room for additional screenshots later
- Screenshots placed in `docs/screenshots/` directory
- Placeholder state: section present but hidden until screenshots are added

### 4. Installation

- Three platforms listed with download buttons/links:
  - **Windows** — `.exe` 安装包
  - **macOS** — `.dmg` 镜像
  - **Linux** — `.tar.gz` 压缩包
- Each entry shows download button with file format label
- Actual download URLs pending publication (placeholder: GitHub Releases link)

### 5. Footer

- Left: "Apache 2.0" + repo link
- Right: "© 2026 uniTerm"

## Technical Decisions

- **Pure static HTML + CSS**: No build tools, no JS framework, no Node dependency. Single HTML file loads fast and GitHub Pages serves it directly.
- **CSS variables**: For theming consistency; reuse app color scheme where practical.
- **No external dependencies**: No Google Fonts, no CDN icons (use inline SVG or emoji for icons). Faster, no tracking, works offline.
- **Responsive**: CSS media queries for mobile layout (single column cards, stacked buttons).

## Files

```
docs/
├── index.html              # Landing page (single file)
├── screenshots/            # App screenshots (user-provided)
│   └── main.png
└── superpowers/
    └── specs/
        └── 2026-05-20-intro-website-design.md
```

## Out of Scope

- PDF documentation or user manual
- Blog or changelog integration
- Search, analytics, or contact forms
