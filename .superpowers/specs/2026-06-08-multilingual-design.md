# 多语言支持设计文档

## 1. 背景与目标

uniTerm 目前已有中英文界面，但翻译消息硬编码在 `frontend/src/i18n/index.ts` 中。用户已将 9 个语言的翻译内容抽出到 `frontend/src/i18n/locales/*.json`：

- `zh-CN` — 简体中文
- `zh-TW` — 繁體中文
- `en` — English
- `ja` — 日本語
- `ko` — 한국어
- `de` — Deutsch
- `es` — Español
- `fr` — Français
- `ru` — Русский

本文档描述如何把这 9 个 JSON 语言包接入现有应用，实现：

1. 启动时全量加载 9 个语言文件；
2. 设置面板可切换全部 9 种语言；
3. 语言切换通过 Vue 响应式机制驱动，组件自动刷新；
4. 清理未引用的翻译 key；
5. 不引入 `vue-i18n` 等新依赖。

## 2. 总体架构

核心改动位于 `frontend/src/i18n/index.ts`：

- 从 9 个 JSON 文件导入消息字典；
- 维护一个全局响应式 `locale` ref；
- 导出 `setLocale()`、`useI18n()`、`t()`；
- 切换语言时更新 ref，依赖 `t()` 的组件自动 re-render。

辅助改动：

- `types/settings.ts`：扩展 `Locale` / `Language` 类型，增加 `LANGUAGE_OPTIONS`；
- `stores/settingsStore.ts`：`updateLanguage()` 中调用 `setLocale()`；
- `components/SettingsTab.vue`：语言下拉框从 `LANGUAGE_OPTIONS` 渲染；
- `main.ts`：应用启动后根据 settings 初始化 locale；
- 各组件：移除 `:key="settingsStore.settings.language"` 强制重渲染 hack。

## 3. 类型与数据模型

### 3.1 Locale / Language

```ts
export const SUPPORTED_LOCALES = [
  'zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'de', 'es', 'fr', 'ru'
] as const

export type Locale = typeof SUPPORTED_LOCALES[number]
export type Language = Locale | 'system'
```

### 3.2 语言选项（供设置页使用）

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

设置页显示 `native` 字段，按上述顺序排列。

`LANGUAGE_OPTIONS` 与 `SUPPORTED_LOCALES` 一同导出，设置页通过 `import { LANGUAGE_OPTIONS } from '../types/settings'` 引用。

## 4. i18n 模块设计

### 4.1 消息加载

启动时一次性静态导入全部 9 个 JSON：

```ts
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
```

### 4.2 全局响应式 locale

```ts
import { ref, computed } from 'vue'

const currentLocale = ref<Locale>('en')

export const locale = computed(() => currentLocale.value)

export function setLocale(lang: Language) {
  currentLocale.value = resolveLocale(lang)
}
```

### 4.3 system 解析策略

```ts
function resolveLocale(lang: Language): Locale {
  if (lang !== 'system') return lang as Locale

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
```

### 4.4 useI18n / t

```ts
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

说明：

- `useI18n().t()` 内部读取 `computed`，在 Vue 响应式上下文中会自动追踪依赖；
- 全局 `t()` 直接读取 `currentLocale.value`，用于非组件同步场景；
- 插值语法保持现有 `{param}` 风格，与 JSON 中占位符一致。

### 4.5 初始化入口

在 `main.ts` 中，Pinia 初始化并加载 settings 后，调用 `setLocale()`：

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

## 5. SettingsStore 改动

### 5.1 updateLanguage

```ts
import { setLocale } from '../i18n'

function updateLanguage(value: AppSettings['language']) {
  settings.value.language = value
  setLocale(value)
  save()
}
```

### 5.2 init 后同步 locale

`settingsStore.init()` 加载完设置后，调用一次 `setLocale(settings.value.language)`，确保启动时界面语言正确。

## 6. 设置页改动

语言下拉框改为循环 `LANGUAGE_OPTIONS`，不再硬编码中英文：

```vue
<el-select
  v-model="settingsStore.settings.language"
  size="small"
  @change="settingsStore.updateLanguage"
>
  <el-option
    v-for="lang in LANGUAGE_OPTIONS"
    :key="lang.value"
    :label="lang.native"
    :value="lang.value"
  />
  <el-option :label="t('settings.langSystem')" value="system" />
</el-select>
```

因为响应式 `t()` 已能驱动刷新，可以移除 `settings-tab` 根节点的 `:key="settingsStore.settings.language"`。

## 7. 翻译 key 清理

实施时执行以下清理：

1. 从 `frontend/src/**/*.{vue,ts}` 中提取所有 `t('key')` / `t("key")` 中的 key；
2. 与 `en.json` 中的 key 集合取差集；
3. 对于未被直接引用的 key，检查是否是动态拼接（如 `monitor.detail.${xxx}`），避免误删；
4. 将确认未使用的 key 从全部 9 个 locale JSON 中删除；
5. 明确删除 `settings.langZhCN` 和 `settings.langEn`，因为设置页改用 `LANGUAGE_OPTIONS` 渲染。

必须保留的 key（即使不被静态 `t('...')` 直接引用）：

- `settings.langSystem`：设置页“系统默认”选项仍使用；
- `monitor.detail.xxx` 系列：通过变量拼接动态访问，需全部保留。

明确删除的 key：

- `settings.langZhCN`
- `settings.langEn`

## 8. 组件 `:key` hack 清理

搜索所有 `:key="settingsStore.settings.language"` 的用法，逐个移除。因为 `useI18n().t()` 基于响应式 ref，语言切换时依赖它的组件会自动 re-render，无需强制 key 变更。

只移除与 language 相关的 `:key`，其他用途的 `:key` 不动。

## 9. Fallback 策略

翻译查找采用三级 fallback：

```ts
const msg = messages[loc]?.[key] ?? messages['en']?.[key] ?? key
```

1. 当前语言；
2. 英语；
3. 直接返回 key（便于开发期识别缺失翻译）。

`system` 解析时，如果浏览器语言不在 9 个支持语言内，默认回退到 `en`。

## 10. Element Plus 语言

Element Plus 组件内部文案（空状态、分页、日期选择器等）**保持英文**，不随应用语言切换。这是为了降低复杂度，避免维护 Element Plus 的 locale 映射和异步加载。

## 11. 文件改动清单

| 文件 | 改动内容 |
|------|----------|
| `frontend/src/i18n/index.ts` | 重写：导入 9 个 JSON、全局 locale ref、`setLocale`、响应式 `t` |
| `frontend/src/types/settings.ts` | 扩展 `Locale`/`Language`、新增 `SUPPORTED_LOCALES`、`LANGUAGE_OPTIONS` |
| `frontend/src/stores/settingsStore.ts` | `updateLanguage` 调用 `setLocale`；`init` 后调用 `setLocale` |
| `frontend/src/main.ts` | 启动时根据 settings 初始化 locale |
| `frontend/src/components/SettingsTab.vue` | 语言下拉框循环 `LANGUAGE_OPTIONS`；移除 `:key` |
| 各组件中的 `:key="settingsStore.settings.language"` | 全部移除 |
| `frontend/src/i18n/locales/*.json` | 删除未引用 key（包括 `settings.langZhCN`、`settings.langEn`） |

## 12. 验证计划

1. **构建检查**
   - `cd frontend && rm -rf dist node_modules/.vite && npm run build` 无报错。

2. **启动行为**
   - 重置设置后首次启动，`language = system`，界面按系统语言显示；
   - 固定语言后重启，保持该语言。

3. **语言切换**
   - 在设置页依次切换到 9 个语言；
   - 界面正常刷新，无白屏/闪烁；
   - `SettingsTab` 等组件不再依赖 `:key` 强制刷新。

4. **Key 完整性**
   - 脚本检查 9 个 JSON key 集合一致；
   - 清理后的 JSON 能正常 import。

5. **Fallback**
   - 临时删除非英语语言的某个 key，切换到该语言，显示英文；
   - 同时删除英文的该 key，显示 raw key。

6. **Element Plus**
   - 确认 Element Plus 组件保持英文，未随应用语言变化。
