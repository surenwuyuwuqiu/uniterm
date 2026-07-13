# uniterm

Wails + Vue（前端 xterm.js 终端）+ Go 后端。

## 约定

- **注释宜少不宜多**：只在意图不明处点一句，不为「改一行代码写五六行注释」。代码自解释优先。
- 提交：PR 标题用英文 conventional commit，正文中英对照；issue 用中文。
- 一分支一修复，基于最新 main。
- 前端改动后 `npm --prefix frontend run build` 验证；终端行为需 `wails dev` 实测。
