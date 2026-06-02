# Merge Tab Bar into Titlebar Design

## Overview

将标签栏合并到标题栏中，节省垂直空间。标题栏（44px）和标签栏（40px）合并为一行，高度约 40-44px。

## Layout

### macOS

```
[窗口控制] [🔗] [+] [   tab1 tab2 ... (center)   ] [AI] [⚙]
```

### Windows

```
[🔗] [+] [tab1 tab2 ... (fill)                   ] [AI] [⚙] [窗口控制]
```

- Tabs 区域撑满剩余空间，无独立拖拽区域（非按钮处均可拖动）
- `···` 溢出菜单仅在标签超出可视区域时显示
- macOS：标签居中；Windows：标签左对齐，自然撑满

## Changes

### AppHeader.vue

| 改动 | 说明 |
|------|------|
| 移除品牌文字 | `.brand` "uniTerm" 删除 |
| Connections 按钮 | 去除文字，仅保留图标 `Network` |
| 新建按钮 | 合并"新建连接"和"本地终端"为一个 `+` 图标下拉按钮。点击默认触发新建连接；展开箭头可选本地终端列表 |
| 设置按钮 | 去除文字，仅保留图标 `Settings` |
| AI 按钮 | 去除文字，仅保留图标 `MessageCircleMore` |
| 嵌入标签栏 | 标签页列表和溢出菜单移入 header |
| 布局调整 | macOS: tabs 居中；Windows: tabs 在 Connections 右侧 |

### TabBar.vue

| 改动 | 说明 |
|------|------|
| 提取标签内容 | tabs-list 和 tab-more 逻辑复用到 AppHeader 中 |
| 保留/移除 | TabBar 组件可删除，或保留为纯逻辑导出 |

### App.vue

| 改动 | 说明 |
|------|------|
| 移除 `<TabBar>` | tab-area 中删除 TabBar 组件渲染 |
| Props/Events | 将 TabBar 相关 props 传给 AppHeader |

### style.css / 组件样式

| 改动 | 说明 |
|------|------|
| `.app-header` 高度 | 保持或微调 |
| `.tab-bar` 高度 | 移除独立 tab-bar 高度占位 |
| 拖拽区域 | header 中间空白区域保持 draggable |

## Non-Goals

- 标签页改为按钮样式：`border-radius`、`padding: 4px 12px`、`font-size: 12px`，与标题栏按钮高度一致
- 不改变标签页切换逻辑
- 不改变拖拽排序能力
- 不影响窗口控制按钮功能
