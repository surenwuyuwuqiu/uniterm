# AI 模型列表同步功能设计文档

**日期**: 2026-06-09
**分支**: `feature/sync-model-list`
**状态**: 设计中

---

## 1. 目标

在 AI 设置页面的模型编辑弹窗中，支持从服务端拉取可用模型列表，简化用户配置。

---

## 2. 需求总结

| 需求项 | 决策 |
|---|---|
| 接口协议 | OpenAI 兼容的 `/v1/models`（`GET`，Bearer Token 鉴权） |
| 触发方式 | 点击按钮手动拉取，拉取成功后模型输入框带下拉建议 |
| UI 交互 | 模型输入框旁加"拉取模型列表"按钮；拉取成功后输入框内显示下拉建议列表；成功/失败均有提示 |
| 表单顺序 | 名称 → Base URL → API Key → 模型（API Key 移至模型前面） |
| 自定义模型 | 支持：下拉选择已有的，也支持手动输入未在列表中的模型 ID |

---

## 3. 交互流程

```
用户打开模型编辑弹窗
    ↓
填写: 名称、Base URL、API Key
    ↓
点击"拉取模型列表"按钮（按钮 loading）
    ↓
前端调用后端 FetchModels(apiKey, baseURL)
    ├─ 成功 → ElMessage.success 提示"已拉取 N 个模型"
    │         模型输入框获得下拉建议列表（el-autocomplete）
    │         用户输入时过滤匹配，点击下拉项填入
    └─ 失败 → ElMessage.error 提示错误信息
              模型输入框仍可手动输入
```

---

## 4. 详细设计

### 4.1 后端 — app.go

新增 Wails 绑定方法：

```go
type ModelInfo struct {
    ID          string `json:"id"`
    DisplayName string `json:"display_name"`
}

// FetchModels 从 OpenAI 兼容的 /v1/models 接口拉取模型列表
func (a *App) FetchModels(apiKey, baseURL string) ([]ModelInfo, error)
```

**实现细节：**
- URL: `strings.TrimRight(baseURL, "/") + "/models"`
- Header: `Authorization: Bearer {apiKey}`
- 解析响应 JSON: `{"object": "list", "data": [{"id": "...", "display_name": "..."}]}`
- 超时: 10 秒
- 错误处理: 网络错误、非 200 状态码均返回 error

### 4.2 前端 — SettingsTab.vue

**表单字段顺序调整：**
```
名称 → Base URL → API Key → 模型（旁边放"拉取模型列表"按钮）
```

**模型行布局：**
```
[el-autocomplete 输入框] [拉取模型列表按钮 (el-button)]
```
- 模型输入框使用 `el-autocomplete`，`fetch-suggestions` 方法从 `modelSuggestions` 过滤
- 初始 `modelSuggestions` 为空，输入框表现为普通文本输入
- 拉取成功后 `modelSuggestions` 有数据，输入框就有下拉建议

**具体改动：**
- 模型字段从 `el-input` 改为 `el-autocomplete`
- 模型字段行增加 `el-button`，文本为"拉取模型列表"，点击触发 `fetchModelList()`
- 拉取成功 → 下拉建议生效 + `ElMessage.success` 提示
- 拉取失败 → `ElMessage.error` 提示
- 用户可手动输入任意模型 ID（不受下拉列表限制）

**新增 data：**
```ts
const modelSuggestions = ref<Array<{ value: string }>>([])
const modelFetching = ref(false)
```

**新增方法：**
```ts
async function fetchModelList() {
  if (!modelForm.apiKey || !modelForm.baseURL) {
    ElMessage.warning(t('settings.fetchModelsHint'))
    return
  }
  modelFetching.value = true
  modelSuggestions.value = []
  try {
    const models = await FetchModels(modelForm.apiKey, modelForm.baseURL)
    modelSuggestions.value = (models || []).map(m => ({
      value: m.displayName || m.id
    }))
    ElMessage.success(t('settings.fetchModelsSuccess', { count: modelSuggestions.value.length }))
  } catch (e: any) {
    ElMessage.error(t('settings.fetchModelsFailed'))
  } finally {
    modelFetching.value = false
  }
}
```

**el-autocomplete 配置：**
```html
<el-autocomplete
  v-model="modelForm.model"
  :fetch-suggestions="(qs, cb) => cb(qs ? modelSuggestions.filter(s => s.value.includes(qs)) : modelSuggestions)"
  :placeholder="'model-id'"
/>
```

### 4.3 Wails Bindings

修改 `app.go` 后需要重新生成前端 bindings：
```bash
wails generate module
```
这会更新 `frontend/wailsjs/go/main/App.js` 和 `frontend/wailsjs/go/main/App.d.ts`。

### 4.4 i18n

新增翻译 key（`frontend/src/i18n/index.ts`）：

| key | 中文 | English |
|-----|------|---------|
| `settings.fetchModels` | 拉取模型列表 | Fetch Models |
| `settings.fetchModelsSuccess` | 已拉取 {count} 个模型 | Fetched {count} models |
| `settings.fetchModelsFailed` | 拉取模型列表失败 | Failed to fetch models |
| `settings.fetchModelsHint` | 请先填写 API Key 和 Base URL | Please fill in API Key and Base URL first |

---

## 5. 涉及文件

| 文件 | 改动类型 |
|---|---|
| `app.go` | 新增 `FetchModels` 方法 |
| `frontend/wailsjs/go/main/App.d.ts` | 自动生成 |
| `frontend/wailsjs/go/main/App.js` | 自动生成 |
| `frontend/src/components/SettingsTab.vue` | 表单顺序调整 + 模型字段改为 el-autocomplete + 拉取按钮 |
| `frontend/src/i18n/index.ts` | 新增 3 个翻译 key |

---

## 6. 验证方式

1. 打开设置 → AI 助理设置 → 添加/编辑模型
2. 填写 Base URL 和 API Key
3. 点击"拉取模型列表"按钮，确认提示"已拉取 N 个模型"，输入框获得下拉建议
4. 输入部分字符过滤，选择一个模型，确认模型 ID 正确填入
5. 手动输入一个不在列表中的模型 ID，确认可以正常保存
6. 填写错误的 Base URL，确认弹错误提示但不阻塞手动输入
