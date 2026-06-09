# AI 模型列表同步功能实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 AI 设置页面的模型编辑弹窗中，添加从 OpenAI 兼容 `/v1/models` 接口拉取可用模型列表的功能。

**Architecture:** Go 后端新增 `FetchModels` Wails 绑定方法，前端 `SettingsTab.vue` 模型编辑弹窗调整表单顺序、将模型输入框改为 `el-autocomplete`、并添加"拉取模型列表"按钮。

**Tech Stack:** Go, Wails v2, Vue 3 + Element Plus, TypeScript

---

### Task 1: 后端 — 新增 FetchModels 方法

**Files:**
- Modify: `app.go`（ChatCompletion 方法之后）

- [ ] **Step 1: 添加 FetchModels 方法和 ModelInfo 结构体**

在 `app.go` 的 `ChatCompletion` 方法之后（约 743 行后），新增以下代码：

```go
// ModelInfo represents a model entry from the /v1/models response.
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// FetchModels fetches the available model list from an OpenAI-compatible /v1/models endpoint.
func (a *App) FetchModels(apiKey, baseURL string) ([]ModelInfo, error) {
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "uniTerm")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
	}

	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}
	return result.Data, nil
}
```

### Task 2: Wails Bindings

**Files:**
- Update: `frontend/wailsjs/go/main/App.js`
- Update: `frontend/wailsjs/go/main/App.d.ts`

- [ ] **Step 1: 重新生成 Wails bindings**

```bash
wails generate module
```

### Task 3: i18n 翻译

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: 在中文 messages 中添加翻译**

在 `'zh-CN'` 对象的 `settings.smartCompletionDesc` 之后（约 234 行）新增：

```ts
'settings.fetchModels': '拉取模型列表',
'settings.fetchModelsSuccess': '已拉取 {count} 个模型',
'settings.fetchModelsFailed': '拉取模型列表失败',
'settings.fetchModelsHint': '请先填写 API Key 和 Base URL',
```

- [ ] **Step 2: 在英文 messages 中添加翻译**

在 `'en'` 对象的 `settings.smartCompletionDesc` 之后（约 788 行）新增：

```ts
'settings.fetchModels': 'Fetch Models',
'settings.fetchModelsSuccess': 'Fetched {count} models',
'settings.fetchModelsFailed': 'Failed to fetch models',
'settings.fetchModelsHint': 'Please fill in API Key and Base URL first',
```

### Task 4: 前端 — SettingsTab.vue 表单改动

**Files:**
- Modify: `frontend/src/components/SettingsTab.vue`

- [ ] **Step 1: 调整表单字段顺序，API Key 移到模型前面**

将弹窗表单（约 360-374 行）从：
```html
<el-form-item :label="t('settings.modelName')">
  <el-input v-model="modelForm.name" />
</el-form-item>
<el-form-item :label="t('settings.modelBaseURL')">
  <el-input v-model="modelForm.baseURL" />
</el-form-item>
<el-form-item :label="t('settings.modelModel')">
  <el-input v-model="modelForm.model" />
</el-form-item>
<el-form-item :label="t('settings.modelApiKey')">
  <el-input v-model="modelForm.apiKey" type="password" show-password />
</el-form-item>
```

改为：
```html
<el-form-item :label="t('settings.modelName')">
  <el-input v-model="modelForm.name" />
</el-form-item>
<el-form-item :label="t('settings.modelBaseURL')">
  <el-input v-model="modelForm.baseURL" />
</el-form-item>
<el-form-item :label="t('settings.modelApiKey')">
  <el-input v-model="modelForm.apiKey" type="password" show-password />
</el-form-item>
<el-form-item :label="t('settings.modelModel')">
  <div class="model-fetch-row">
    <el-autocomplete
      v-model="modelForm.model"
      :fetch-suggestions="(qs, cb) => cb(qs ? modelSuggestions.filter(s => s.value.toLowerCase().includes(qs.toLowerCase())) : modelSuggestions)"
      class="model-autocomplete"
    />
    <el-button size="small" :loading="modelFetching" @click="fetchModelList">
      {{ t('settings.fetchModels') }}
    </el-button>
  </div>
</el-form-item>
```

- [ ] **Step 2: 在 script setup 中添加 import**

在 `import { ElMessage } from 'element-plus'` 之后添加：
```ts
import { FetchModels } from '../../wailsjs/go/main/App'
```

- [ ] **Step 3: 在 script setup 中添加响应式变量**

在 `const showModelForm = ref(false)` 之后添加：
```ts
const modelSuggestions = ref<Array<{ value: string }>>([])
const modelFetching = ref(false)
```

- [ ] **Step 4: 在 editModel 函数中清空建议列表**

修改 `editModel` 函数（约 529 行），在 `showModelForm.value = true` 之前添加：
```ts
modelSuggestions.value = []
```

- [ ] **Step 5: 在 saveModel 后清空建议**

修改 `saveModel` 函数（约 535 行），在 `showModelForm.value = false` 之后、`editingModel.value = null` 之前添加：
```ts
modelSuggestions.value = []
```

实际上在 `resetModelForm` 之后清空更简单。直接在 `resetModelForm`（约 552 行）函数末尾添加：
```ts
modelSuggestions.value = []
```

- [ ] **Step 6: 在 script setup 末尾添加 fetchModelList 方法**

在 `getShellLabel` 函数之前（约 560 行前）添加：

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

- [ ] **Step 7: 添加 model-fetch-row 样式**

在 `<style scoped>` 末尾（约 1002 行前）添加：

```css
.model-fetch-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
.model-autocomplete {
  flex: 1;
}
```

### Task 5: 编译验证

- [ ] **Step 1: 清理前端缓存并构建**

```bash
cd frontend && rm -rf dist node_modules/.vite && npm run build && cd ..
```

- [ ] **Step 2: 编译 Windows**

```bash
wails build -platform windows/amd64
```

- [ ] **Step 3: 验证功能**

1. 启动 `build/bin/uniTerm.exe`
2. 打开设置 → AI 助理设置 → 点击"添加模型"
3. 确认表单顺序为：名称 → Base URL → API Key → 模型（带拉取按钮）
4. 填写 Base URL `https://t.mysugoncloud.com:8765/v1` 和 API Key
5. 点击"拉取模型列表"，确认提示"已拉取 17 个模型"，模型输入框获得下拉建议
6. 在模型输入框输入"claude"，确认下拉列表过滤只显示匹配项
7. 选择一个模型，确认 ID 填入
8. 不填写 API Key 直接点击按钮，确认提示"请先填写 API Key 和 Base URL"
