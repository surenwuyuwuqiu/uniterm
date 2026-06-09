# 多语言支持实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 9 个 JSON 语言包接入 uniTerm，实现设置面板切换全部 9 种语言，并通过响应式 locale 驱动界面自动刷新。

**Architecture:** 在 `frontend/src/i18n/index.ts` 中维护全局响应式 `locale` ref，启动时静态导入 9 个 JSON 合并为消息字典；`settingsStore` 切换语言时调用 `setLocale()` 更新 ref，所有使用 `useI18n().t()` 的组件自动重新渲染。

**Tech Stack:** Vue 3, Pinia, Vite, Element Plus, TypeScript, Wails

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `frontend/src/types/settings.ts` | 扩展 `Locale`/`Language` 类型，导出 `SUPPORTED_LOCALES` 和 `LANGUAGE_OPTIONS` |
| `frontend/src/i18n/index.ts` | i18n 核心：导入 9 个 JSON、全局 `locale` ref、`setLocale`、`useI18n`、`t` |
| `frontend/src/stores/settingsStore.ts` | `updateLanguage` 调用 `setLocale`；`init` 后同步 locale |
| `frontend/src/main.ts` | 启动时初始化 locale |
| `frontend/src/components/SettingsTab.vue` | 语言下拉框循环 `LANGUAGE_OPTIONS`；移除 `:key` hack |
| `frontend/src/i18n/locales/*.json` | 9 个语言文件；清理未引用 key |

---

## Task 1: 扩展类型与语言选项常量

**Files:**
- Modify: `frontend/src/types/settings.ts`

- [ ] **Step 1.1: 替换 Language / Locale 类型，新增常量**

将文件开头的类型定义和常量扩展：

```ts
export const SUPPORTED_LOCALES = [
  'zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'de', 'es', 'fr', 'ru'
] as const

export type Locale = typeof SUPPORTED_LOCALES[number]
export type Language = Locale | 'system'
```

替换原有：
```ts
export type Language = 'zh-CN' | 'en' | 'system'
```

并在文件末尾（`RIGHT_CLICK_ACTIONS` 之后）新增：

```ts
export const LANGUAGE_OPTIONS: { value: Locale; label: string; native: string }[] = [
  { value: 'zh-CN', label: '简体中文', native: '简体中文' },
  { value: 'zh-TW', label: '繁體中文', native: '繁體中文' },
  { value: 'en', label: 'English', native: 'English' },
  { value: 'ja', label: '日本語', native: '日本語' },
  { value: 'ko', label: '한국어', native: '한국어' },
  { value: 'de', label: 'Deutsch', native: 'Deutsch' },
  { value: 'es', label: 'Español', native: 'Español' },
  { value: 'fr', label: 'Français', native: 'Français' },
  { value: 'ru', label: 'Русский', native: 'Русский' },
]
```

- [ ] **Step 1.2: 验证 TypeScript 无报错**

Run: `cd frontend && npx vue-tsc --noEmit`

Expected: 无报错（如果项目未配置 `vue-tsc`，则跳过此步）。

---

## Task 2: 重写 i18n 核心模块

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 2.1: 重写整个文件为响应式实现**

替换 `frontend/src/i18n/index.ts` 全部内容为：

```ts
import { computed, ref } from 'vue'
import type { Language, Locale } from '../types/settings'
import { SUPPORTED_LOCALES } from '../types/settings'

import zhCN from './locales/zh-CN.json'
import zhTW from './locales/zh-TW.json'
import en from './locales/en.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import de from './locales/de.json'
import es from './locales/es.json'
import fr from './locales/fr.json'
import ru from './locales/ru.json'

const messages: Record<Locale, Record<string, string>> = {
  'zh-CN': zhCN,
  'zh-TW': zhTW,
  en,
  ja,
  ko,
  de,
  es,
  fr,
  ru,
}

const currentLocale = ref<Locale>('en')

export const locale = computed(() => currentLocale.value)

function resolveLocale(lang: Language): Locale {
  if (lang !== 'system') {
    if (SUPPORTED_LOCALES.includes(lang as Locale)) return lang as Locale
    return 'en'
  }

  const nav = navigator.language.toLowerCase()

  if (nav.startsWith('zh')) {
    if (nav.includes('tw') || nav.includes('hk') || nav.includes('mo')) return 'zh-TW'
    return 'zh-CN'
  }

  const map: Record<string, Locale> = {
    en: 'en', ja: 'ja', ko: 'ko',
    de: 'de', es: 'es', fr: 'fr', ru: 'ru',
  }
  for (const [prefix, loc] of Object.entries(map)) {
    if (nav.startsWith(prefix)) return loc
  }

  return 'en'
}

export function setLocale(lang: Language) {
  currentLocale.value = resolveLocale(lang)
}

export function useI18n() {
  const localeSnapshot = computed(() => currentLocale.value)

  function t(key: string, params?: Record<string, string | number>): string {
    const loc = localeSnapshot.value
    const msg = messages[loc]?.[key] ?? messages['en']?.[key] ?? key
    if (!params) return msg
    return msg.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`))
  }

  return { t, locale: localeSnapshot }
}

export function t(key: string, params?: Record<string, string | number>): string {
  const loc = currentLocale.value
  const msg = messages[loc]?.[key] ?? messages['en']?.[key] ?? key
  if (!params) return msg
  return msg.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`))
}
```

- [ ] **Step 2.2: 验证 i18n 模块能独立编译**

Run: `cd frontend && npx vue-tsc --noEmit`

Expected: 无报错（可能有其他文件报错，但 i18n/index.ts 本身应无错）。

---

## Task 3: SettingsStore 接入 setLocale

**Files:**
- Modify: `frontend/src/stores/settingsStore.ts`

- [ ] **Step 3.1: 导入 setLocale 并在 updateLanguage 中调用**

在文件顶部新增导入：

```ts
import { setLocale } from '../i18n'
```

修改 `updateLanguage` 函数：

```ts
function updateLanguage(value: AppSettings['language']) {
  settings.value.language = value
  setLocale(value)
  save()
}
```

- [ ] **Step 3.2: init 完成后同步 locale**

在 `init()` 函数末尾、`applyTheme()` 之后新增：

```ts
setLocale(settings.value.language)
```

完整 `init()` 修改后末尾类似：

```ts
applyTheme()
setLocale(settings.value.language)
```

- [ ] **Step 3.3: 验证 TypeScript**

Run: `cd frontend && npx vue-tsc --noEmit`

Expected: 无新增报错。

---

## Task 4: 应用启动时初始化 locale

**Files:**
- Modify: `frontend/src/main.ts`

- [ ] **Step 4.1: 导入 setLocale 和 useSettingsStore**

在文件顶部新增：

```ts
import { setLocale } from './i18n'
import { useSettingsStore } from './stores/settingsStore'
```

- [ ] **Step 4.2: 在 mount 前初始化 locale**

将：

```ts
const app = createApp(App)
app.use(createPinia())
app.use(ElementPlus)
app.mount('#app')
```

改为：

```ts
const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(ElementPlus)

const settingsStore = useSettingsStore()
await settingsStore.init()
setLocale(settingsStore.settings.language)

app.mount('#app')
```

注意：`WindowSetTitle` 调用可以保留在原有位置（它在 `app.mount` 之前或之后均可）。

- [ ] **Step 4.3: 验证 TypeScript**

Run: `cd frontend && npx vue-tsc --noEmit`

Expected: 无新增报错。

---

## Task 5: 设置页语言下拉框改造

**Files:**
- Modify: `frontend/src/components/SettingsTab.vue`

- [ ] **Step 5.1: 导入 LANGUAGE_OPTIONS**

在 `<script setup>` 顶部找到现有导入，新增：

```ts
import { LANGUAGE_OPTIONS } from '../types/settings'
```

- [ ] **Step 5.2: 移除根节点 :key**

将第 2 行：

```vue
<div class="settings-tab" :key="settingsStore.settings.language">
```

改为：

```vue
<div class="settings-tab">
```

- [ ] **Step 5.3: 替换语言下拉框**

将：

```vue
<el-select v-model="settingsStore.settings.language" size="small" @change="settingsStore.save()">
  <el-option :label="t('settings.langZhCN')" value="zh-CN" />
  <el-option :label="t('settings.langEn')" value="en" />
  <el-option :label="t('settings.langSystem')" value="system" />
</el-select>
```

改为：

```vue
<el-select v-model="settingsStore.settings.language" size="small" @change="settingsStore.updateLanguage">
  <el-option
    v-for="lang in LANGUAGE_OPTIONS"
    :key="lang.value"
    :label="lang.native"
    :value="lang.value"
  />
  <el-option :label="t('settings.langSystem')" value="system" />
</el-select>
```

- [ ] **Step 5.4: 验证 TypeScript**

Run: `cd frontend && npx vue-tsc --noEmit`

Expected: 无新增报错。

---

## Task 6: 扫描并清理未引用翻译 key

**Files:**
- Modify: `frontend/src/i18n/locales/zh-CN.json`
- Modify: `frontend/src/i18n/locales/zh-TW.json`
- Modify: `frontend/src/i18n/locales/en.json`
- Modify: `frontend/src/i18n/locales/ja.json`
- Modify: `frontend/src/i18n/locales/ko.json`
- Modify: `frontend/src/i18n/locales/de.json`
- Modify: `frontend/src/i18n/locales/es.json`
- Modify: `frontend/src/i18n/locales/fr.json`
- Modify: `frontend/src/i18n/locales/ru.json`

- [ ] **Step 6.1: 编写临时扫描脚本**

在项目根目录创建 `scripts/find-unused-i18n-keys.js`：

```js
const fs = require('fs')
const path = require('path')

const srcDir = path.join(__dirname, '../frontend/src')
const localesDir = path.join(srcDir, 'i18n/locales')

function extractKeys(dir, exts) {
  const keys = new Set()
  const files = []
  function walk(d) {
    for (const entry of fs.readdirSync(d, { withFileTypes: true })) {
      const full = path.join(d, entry.name)
      if (entry.isDirectory()) walk(full)
      else if (exts.some((e) => entry.name.endsWith(e))) files.push(full)
    }
  }
  walk(dir)

  const re = /t\(['"]([^'"]+)['"]\)/g
  for (const f of files) {
    const text = fs.readFileSync(f, 'utf8')
    let m
    while ((m = re.exec(text)) !== null) keys.add(m[1])
  }
  return keys
}

const usedKeys = extractKeys(srcDir, ['.vue', '.ts'])
const en = JSON.parse(fs.readFileSync(path.join(localesDir, 'en.json'), 'utf8'))
const allKeys = Object.keys(en)

const unused = allKeys.filter((k) => !usedKeys.has(k))
const preserved = [
  'settings.langSystem',
  'monitor.detail.pid', 'monitor.detail.ppid', 'monitor.detail.state',
  'monitor.detail.threads', 'monitor.detail.exe', 'monitor.detail.cwd',
  'monitor.detail.cmdline', 'monitor.detail.startTime', 'monitor.detail.fd',
  'monitor.detail.fdTotal', 'monitor.detail.fdFiles', 'monitor.detail.fdSockets',
  'monitor.detail.fdPipes', 'monitor.detail.fdAnons', 'monitor.detail.fdDevs',
  'monitor.detail.fdOthers', 'monitor.detail.vmRss', 'monitor.detail.vmSize',
  'monitor.detail.io', 'monitor.detail.cpuTicks', 'monitor.detail.ctxSwitches',
]

const toDelete = unused.filter((k) => !preserved.some((p) => k === p || k.startsWith(p + '.')))

console.log('Used keys:', usedKeys.size)
console.log('Total keys:', allKeys.length)
console.log('Unused keys:', unused.length)
console.log('Keys to delete:', toDelete.length)
console.log('\nTo delete:')
toDelete.forEach((k) => console.log('  -', k))
```

Run: `node scripts/find-unused-i18n-keys.js`

Expected: 输出待删除 key 列表，其中应包含 `settings.langZhCN` 和 `settings.langEn`。

- [ ] **Step 6.2: 从全部 9 个 JSON 中删除未引用 key**

根据脚本输出，手动（或继续用脚本）从全部 9 个 locale JSON 中删除对应 key。

若要用脚本自动删除，在同一文件末尾追加并执行：

```js
const localeFiles = fs.readdirSync(localesDir).filter((f) => f.endsWith('.json'))
for (const file of localeFiles) {
  const p = path.join(localesDir, file)
  const data = JSON.parse(fs.readFileSync(p, 'utf8'))
  for (const k of toDelete) delete data[k]
  fs.writeFileSync(p, JSON.stringify(data, null, 2) + '\n')
}
console.log('Cleaned', localeFiles.length, 'files')
```

- [ ] **Step 6.3: 验证 9 个 JSON key 集合一致**

Run:

```bash
cd frontend/src/i18n/locales
node -e "
const fs = require('fs');
const files = fs.readdirSync('.').filter(f => f.endsWith('.json'));
const keys = files.map(f => Object.keys(JSON.parse(fs.readFileSync(f,'utf8'))).sort().join(','));
const first = keys[0];
files.forEach((f,i) => {
  if (keys[i] !== first) console.log('MISMATCH', f);
});
console.log('Checked', files.length, 'files');
"
```

Expected: 无 `MISMATCH` 输出。

- [ ] **Step 6.4: 删除临时脚本（可选）**

如果不需要保留扫描脚本：

Run: `rm scripts/find-unused-i18n-keys.js && rmdir scripts 2>/dev/null || true`

---

## Task 7: 构建与启动验证

- [ ] **Step 7.1: 前端构建**

Run:

```bash
cd frontend
rm -rf dist node_modules/.vite
npm run build
```

Expected: 构建成功，无 TypeScript / Vite 报错。

- [ ] **Step 7.2: 启动桌面应用**

Run:

```bash
cd ..
wails dev
```

Expected: 应用正常启动，界面语言与系统语言或已保存设置一致。

- [ ] **Step 7.3: 语言切换测试**

1. 打开设置页 → 基础设置 → 语言。
2. 依次切换到 `zh-CN`、`zh-TW`、`en`、`ja`、`ko`、`de`、`es`、`fr`、`ru`、`system`。
3. 每次切换后观察：设置页文案是否正确变化、是否有闪烁或白屏、下拉框本身文字是否正确。

Expected: 9 种语言切换均正常，无需 `:key` 强制刷新也能即时更新。

- [ ] **Step 7.4: Fallback 测试**

1. 临时在 `frontend/src/i18n/locales/de.json` 中删除任意一个 key（例如 `"header.ai"`）。
2. 切换到德语，观察该 key 对应位置是否显示英文 `AI`。
3. 同时从 `en.json` 中也删除该 key，观察是否显示 raw key `header.ai`。
4. 测试结束后恢复删除（或重新从原始文件还原）。

- [ ] **Step 7.5: 首次启动（system 语言）测试**

1. 关闭应用。
2. 临时将系统语言改为非中文非英文（如日语）。
3. 删除/重命名应用设置缓存（位置取决于 Wails 存储实现，通常为 OS 级应用数据目录）。
4. 重新启动应用。

Expected: 首次启动时界面显示日语（或最接近的 supported locale），因为 setting 默认为 `system`。

- [ ] **Step 7.6: Element Plus 保持英文验证**

触发 Element Plus 组件，如下拉框空状态、消息提示 `ElMessage` 等。

Expected: Element Plus 组件内部文案保持英文，不受应用语言切换影响。

---

## Task 8: 最终检查与收尾

- [ ] **Step 8.1: 确认没有残留的 `:key="settingsStore.settings.language"`**

Run:

```bash
grep -rn ':key="settingsStore.settings.language"' frontend/src || echo "None found"
```

Expected: 输出 `None found`。

- [ ] **Step 8.2: 确认没有残留的对 `settings.langZhCN` / `settings.langEn` 的引用**

Run:

```bash
grep -rn "settings\.langZhCN\|settings\.langEn" frontend/src || echo "None found"
```

Expected: 输出 `None found`。

- [ ] **Step 8.3: 最终构建验证**

Run:

```bash
cd frontend && rm -rf dist node_modules/.vite && npm run build
```

Expected: 构建成功。

---

## 自检清单

- [x] Spec 覆盖：9 语言加载、响应式 locale、SettingsStore 联动、设置页改造、key 清理、:key hack 移除、Element Plus 保持英文、验证计划均有对应任务。
- [x] 无占位符：每步包含具体代码或命令。
- [x] 类型一致：`Locale`、`Language`、`LANGUAGE_OPTIONS`、`SUPPORTED_LOCALES` 在 Task 1 定义，后续任务引用一致。
- [x] 文件路径使用绝对相对路径：`frontend/src/...`。
