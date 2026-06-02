# SPICE 远程桌面实施计划

> **Goal:** 为 uniTerm 新增 SPICE 远程桌面协议支持，架构与 VNC 完全对标。

**Architecture:** Go 后端 SPICEProxy (WebSocket↔TCP 桥接) + spice-html5 前端 Canvas 渲染。

**Tech Stack:** Go + Vue 3 + spice-html5 + gorilla/websocket

---

### Task 1: Go 后端 — SPICEProxy

创建 `backend/session/spice_proxy.go`，与 VNCProxy 几乎相同。

### Task 2: Go 后端 — SPICESession

创建 `backend/session/spice_session.go`，实现 Session 接口。

### Task 3: Go 后端 — manager.go + app.go

注册 SPICE 会话类型，连接时传递 proxyAddr。

### Task 4: 前端 — 类型定义

更新 session.ts 和 workspace.ts 的类型。

### Task 5: 前端 — SPICETabContent.vue

创建 SPICE 标签页组件。

### Task 6: 前端 — i18n + ConnectionForm + Sidebar + tabStore + panelStore + App.vue

所有前端集成点。
