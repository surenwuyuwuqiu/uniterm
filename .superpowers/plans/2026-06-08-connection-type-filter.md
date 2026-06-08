# 连接类型筛选器实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Sidebar 搜索框的 suffix 位置添加一个连接类型筛选按钮，支持下拉选择类型并实时过滤连接列表。

**Architecture:** 在 Sidebar.vue 中新增筛选状态管理和 UI 组件。使用 `el-dropdown` 实现下拉菜单，动态从 `connectionStore.connections` 提取可用类型（database 按 dbType 细分）。修改 `filteredGrouped` computed 同时应用文本搜索和类型筛选（AND 逻辑）。

**Tech Stack:** Vue 3, Element Plus, Pinia, @lucide/vue

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `frontend/src/i18n/index.ts` | 修改 | 添加 `sidebar.filterAll` 中英文翻译键 |
| `frontend/src/components/Sidebar.vue` | 修改 | 添加筛选状态、类型提取逻辑、下拉 UI、修改过滤逻辑 |

---

### Task 1: 添加 i18n 翻译键

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: 在 zh-CN 区域添加翻译键**

在 `frontend/src/i18n/index.ts` 的 `zh-CN` 区域的 `// Sidebar` 注释下，`'sidebar.newConnectionFromSearch'` 行之后添加：

```typescript
    'sidebar.filterAll': '全部',
```

- [ ] **Step 2: 在 en 区域添加翻译键**

在 `frontend/src/i18n/index.ts` 的 `en` 区域的 `// Sidebar` 注释下，`'sidebar.newConnectionFromSearch'` 行之后添加：

```typescript
    'sidebar.filterAll': 'All',
```

- [ ] **Step 3: 验证**

检查两个语言区域都添加了相同的键名 `sidebar.filterAll`。

---

### Task 2: 添加筛选状态管理和类型提取逻辑

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: 导入 Filter 和 Check 图标**

修改 Sidebar.vue 顶部的 import 语句，从 `@lucide/vue` 导入 `Filter` 和 `Check`：

```typescript
import { X, ChevronRight, ChevronDown, Filter, Check } from '@lucide/vue'
```

- [ ] **Step 2: 添加筛选状态 ref**

在 `searchQuery` ref 定义之后（约第 325 行），添加：

```typescript
const selectedTypeFilter = ref('all')
```

- [ ] **Step 3: 添加可用类型列表 computed**

在 `selectedTypeFilter` 之后，添加类型提取 computed：

```typescript
interface TypeOption {
  label: string
  value: string
}

const TYPE_LABELS: Record<string, string> = {
  ssh: 'SSH',
  telnet: 'Telnet',
  mosh: 'Mosh',
  rdp: 'RDP',
  vnc: 'VNC',
  spice: 'SPICE',
  local: 'Local',
  sftp: 'SFTP',
  monitor: 'Monitor',
  'database:mysql': 'MySQL',
  'database:postgres': 'PostgreSQL',
  'database:rqlite': 'rqlite',
}

const availableTypes = computed<TypeOption[]>(() => {
  const types = new Set<string>()
  for (const c of connectionStore.connections) {
    if (c.type === 'database' && c.dbType) {
      types.add(`database:${c.dbType}`)
    } else {
      types.add(c.type)
    }
  }
  return [...types].sort().map(value => ({
    value,
    label: TYPE_LABELS[value] || value
  }))
})
```

- [ ] **Step 4: 添加类型匹配辅助函数**

在 `availableTypes` computed 之后，添加：

```typescript
function matchTypeFilter(conn: ConnectionConfig, filter: string): boolean {
  if (filter === 'all') return true
  if (filter.startsWith('database:')) {
    return conn.type === 'database' && conn.dbType === filter.slice(9)
  }
  return conn.type === filter
}
```

- [ ] **Step 5: 验证编译**

运行以下命令确保没有 TypeScript 错误：

```bash
cd frontend && npx vue-tsc --noEmit --skipLibCheck
```

Expected: 无错误输出。

---

### Task 3: 修改搜索框添加 suffix 筛选按钮和下拉菜单

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: 修改搜索框添加 suffix slot**

将 Sidebar.vue 中现有的搜索框代码（约第 18-25 行）：

```vue
    <div class="search-box">
      <el-input
        v-model="searchQuery"
        :placeholder="t('sidebar.searchPlaceholder')"
        clearable
        @keydown="onListKeydown"
      />
    </div>
```

替换为：

```vue
    <div class="search-box">
      <el-input
        v-model="searchQuery"
        :placeholder="t('sidebar.searchPlaceholder')"
        clearable
        @keydown="onListKeydown"
      >
        <template #suffix>
          <el-dropdown trigger="click" placement="bottom-end" :teleported="false">
            <span
              class="filter-trigger"
              :class="{ active: selectedTypeFilter !== 'all' }"
              @click.stop
            >
              <el-icon><Filter :size="14" /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu class="type-filter-menu">
                <el-dropdown-item
                  :class="{ 'is-active': selectedTypeFilter === 'all' }"
                  @click="selectedTypeFilter = 'all'"
                >
                  <span class="dropdown-item-content">
                    <el-icon v-if="selectedTypeFilter === 'all'"><Check :size="14" /></el-icon>
                    <span v-else class="check-placeholder"></span>
                    <span>{{ t('sidebar.filterAll') }}</span>
                  </span>
                </el-dropdown-item>
                <el-dropdown-item divided v-if="availableTypes.length > 0" />
                <el-dropdown-item
                  v-for="typeOpt in availableTypes"
                  :key="typeOpt.value"
                  :class="{ 'is-active': selectedTypeFilter === typeOpt.value }"
                  @click="selectedTypeFilter = typeOpt.value"
                >
                  <span class="dropdown-item-content">
                    <el-icon v-if="selectedTypeFilter === typeOpt.value"><Check :size="14" /></el-icon>
                    <span v-else class="check-placeholder"></span>
                    <span>{{ typeOpt.label }}</span>
                  </span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-input>
    </div>
```

注意：`el-dropdown` 的 `:teleported="false"` 确保下拉菜单在 sidebar 内部渲染，避免被其他元素遮挡。

- [ ] **Step 2: 验证编译**

```bash
cd frontend && npx vue-tsc --noEmit --skipLibCheck
```

Expected: 无错误输出。

---

### Task 4: 修改过滤逻辑同时应用搜索和类型筛选

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: 修改 filteredGrouped computed**

将现有的 `filteredGrouped` computed（约第 343-369 行）替换为：

```typescript
const filteredGrouped = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const typeFilter = selectedTypeFilter.value
  const data = connectionStore.groupedConnections

  const matchConn = (c: ConnectionConfig) => {
    const textMatch = !q || c.name.toLowerCase().includes(q) || c.host.toLowerCase().includes(q)
    const typeMatch = matchTypeFilter(c, typeFilter)
    return textMatch && typeMatch
  }

  const filteredGroups = data.groups
    .map(entry => ({
      group: entry.group,
      connections: entry.connections.filter(matchConn)
    }))
    .filter(entry => {
      const groupNameMatch = entry.group.name.toLowerCase().includes(q)
      if (groupNameMatch) {
        // Show all connections in group when group name matches, but still apply type filter
        entry.connections = data.groups.find(g => g.group.id === entry.group.id)!.connections.filter(matchConn)
        return true
      }
      return entry.connections.length > 0
    })

  const filteredUngrouped = data.ungrouped.filter(matchConn)

  return { groups: filteredGroups, ungrouped: filteredUngrouped }
})
```

注意：当分组名称匹配搜索文本时，仍然需要应用类型筛选（修正原逻辑中类型筛选被跳过的问题）。

- [ ] **Step 2: 更新 watch(filteredGrouped) 中的空状态处理**

检查现有的 `watch(filteredGrouped, ...)`（约第 381-395 行），确保当筛选结果为空时正确重置选中状态。现有逻辑已能正确处理，无需修改。

- [ ] **Step 3: 验证编译**

```bash
cd frontend && npx vue-tsc --noEmit --skipLibCheck
```

Expected: 无错误输出。

---

### Task 5: 添加样式

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: 在 style scoped 区域添加筛选按钮和下拉菜单样式**

在 Sidebar.vue 的 `<style scoped>` 区域末尾（在 `.virtual-new-conn .virtual-name` 规则之后，约第 1285 行）添加：

```css
.filter-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--text-muted);
  transition: color 0.12s ease;
  padding: 2px;
  border-radius: var(--radius-sm);
}

.filter-trigger:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.filter-trigger.active {
  color: var(--accent);
}

.filter-trigger.active:hover {
  color: var(--accent);
  background: var(--accent-subtle);
}
```

- [ ] **Step 2: 在全局 style 区域添加下拉菜单样式**

在 Sidebar.vue 的 `<style>`（非 scoped）区域末尾（在 `.conn-context-menu .menu-divider` 规则之后，约第 1332 行）添加：

```css
.type-filter-menu .el-dropdown-menu__item {
  padding: 6px 12px;
  font-size: 12px;
  font-family: var(--font-ui);
}

.type-filter-menu .el-dropdown-menu__item.is-active {
  color: var(--accent);
}

.dropdown-item-content {
  display: flex;
  align-items: center;
  gap: 6px;
}

.check-placeholder {
  display: inline-block;
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}
```

- [ ] **Step 3: 验证编译**

```bash
cd frontend && npx vue-tsc --noEmit --skipLibCheck
```

Expected: 无错误输出。

---

### Task 6: 启动开发服务器验证功能

**Files:**
- 无需修改文件

- [ ] **Step 1: 清理缓存并启动开发服务器**

```bash
cd frontend && rm -rf dist node_modules/.vite && cd .. && wails dev
```

- [ ] **Step 2: 手动验证功能**

1. 打开应用，查看 sidebar 搜索框右侧是否出现筛选图标
2. 点击筛选图标，确认下拉菜单显示 "全部" 和当前连接列表中的类型
3. 选择一个类型，确认连接列表只显示该类型的连接
4. 在搜索框输入文本，确认同时应用文本搜索和类型筛选
5. 选择 "全部"，确认恢复显示所有连接
6. 验证筛选图标在选中非 "全部" 时变为 accent 色
7. 验证键盘导航（上下箭头、Enter）在筛选后的列表中正常工作

---

## 自检清单

**Spec 覆盖：**
- [x] 搜索框右侧添加筛选按钮 — Task 3
- [x] 默认 ALL — Task 2 (`selectedTypeFilter = 'all'`)
- [x] 下拉显示当前连接列表中的类型 — Task 2 (`availableTypes` computed)
- [x] database 按 dbType 细分 — Task 2 (`database:${dbType}` 格式)
- [x] 点击后筛选对应类型 — Task 4 (`filteredGrouped` 修改)
- [x] 搜索和类型同时生效（AND）— Task 4
- [x] 不显示数量 — Task 2/3 (未添加数量显示)

**Placeholder 扫描：** 无 TBD、TODO 或模糊描述。

**类型一致性：** `TypeOption` 接口、`TYPE_LABELS` 映射、`matchTypeFilter` 函数签名在 Task 2 中定义，在 Task 4 中使用，一致。
