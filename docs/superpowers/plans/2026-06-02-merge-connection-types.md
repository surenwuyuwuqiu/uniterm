# Merge Connection Types Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merge SSH/Telnet/Mosh into a single "终端" category with two-level radio-button selection in ConnectionForm.vue, alongside "远程桌面" (RDP/VNC) and "数据库" (MySQL/PostgreSQL/rqlite). Pure frontend change.

**Architecture:** Add a `category` computed property in ConnectionForm.vue that maps `form.type` to one of three categories. Replace the flat `<el-radio-group>` with two levels: category radio-buttons filter which protocol radio-buttons are shown. Switching category auto-selects the first protocol in that category and updates the default port.

**Tech Stack:** Vue 3, Element Plus, TypeScript

---

### Task 1: Add i18n labels for categories

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: Add Chinese category labels**

In the `zh` object, add under `conn`:
```typescript
'conn.categoryTerminal': '终端',
'conn.categoryRemote': '远程桌面',
```

No new key needed for "数据库" — reuse existing `'db.database': '数据库'`.

- [ ] **Step 2: Add English category labels**

In the `en` object, add under `conn`:
```typescript
'conn.categoryTerminal': 'Terminal',
'conn.categoryRemote': 'Remote',
```

---

### Task 2: Refactor ConnectionForm.vue template — two-level type selector

**Files:**
- Modify: `frontend/src/components/ConnectionForm.vue:25-41`

- [ ] **Step 1: Replace the flat radio-group with two-level structure**

Replace:
```html
<el-form-item :label="t('conn.type')">
  <el-radio-group v-model="form.type">
    <el-radio-button label="ssh">SSH</el-radio-button>
    <el-radio-button label="telnet">Telnet</el-radio-button>
    <el-radio-button label="mosh">Mosh</el-radio-button>
    <el-radio-button label="rdp" v-if="isWindows">RDP</el-radio-button>
    <el-radio-button label="vnc">VNC</el-radio-button>
    <el-radio-button label="database">{{ t('db.database') }}</el-radio-button>
  </el-radio-group>
</el-form-item>
<el-form-item v-if="form.type === 'database'" :label="t('db.dbType')">
  <el-select v-model="form.dbType" :placeholder="t('db.dbType')">
    <el-option label="MySQL" value="mysql" />
    <el-option label="PostgreSQL" value="postgres" />
    <el-option label="rqlite" value="rqlite" />
  </el-select>
</el-form-item>
```

With:
```html
<el-form-item :label="t('conn.type')">
  <el-radio-group v-model="category" @change="onCategoryChange">
    <el-radio-button value="terminal">{{ t('conn.categoryTerminal') }}</el-radio-button>
    <el-radio-button value="remote">{{ t('conn.categoryRemote') }}</el-radio-button>
    <el-radio-button value="database">{{ t('db.database') }}</el-radio-button>
  </el-radio-group>
</el-form-item>
<el-form-item v-if="category" label="">
  <el-radio-group v-model="form.type" @change="onProtocolChange">
    <template v-if="category === 'terminal'">
      <el-radio-button label="ssh">SSH</el-radio-button>
      <el-radio-button label="telnet">Telnet</el-radio-button>
      <el-radio-button label="mosh">Mosh</el-radio-button>
    </template>
    <template v-if="category === 'remote'">
      <el-radio-button label="rdp" v-if="isWindows">RDP</el-radio-button>
      <el-radio-button label="vnc">VNC</el-radio-button>
    </template>
    <template v-if="category === 'database'">
      <el-radio-button label="database" value="mysql" @click="onDbTypeSelect('mysql')">MySQL</el-radio-button>
      <el-radio-button label="database" value="postgres" @click="onDbTypeSelect('postgres')">PostgreSQL</el-radio-button>
      <el-radio-button label="database" value="rqlite" @click="onDbTypeSelect('rqlite')">rqlite</el-radio-button>
    </template>
  </el-radio-group>
</el-form-item>
```

Wait — database is tricky because `form.type` is `'database'` for all three, and the sub-type is in `form.dbType`. Using `el-radio-button` with the same `label` but different values won't work because `label` drives the active state.

Better approach: use a separate reactive `selectedDbType` for the database toggle buttons, and sync it with `form.dbType` + set `form.type = 'database'`.

Actually, the cleanest approach for database:

```html
<template v-if="category === 'database'">
  <el-radio-button
    v-for="db in dbOptions"
    :key="db.value"
    :label="db.value"
    @click="onDbTypeSelect(db.value)"
  >{{ db.label }}</el-radio-button>
</template>
```

Hmm, but `el-radio-button` works with `el-radio-group`'s `v-model`. If `v-model` is `form.type`, clicking any database option sets `form.type` to that value. But we need `form.type = 'database'` for all of them, with `form.dbType` varying.

Simpler approach: use a manual button group for database, not `el-radio-group`:

```html
<template v-if="category === 'database'">
  <el-radio-group v-model="form.dbType">
    <el-radio-button label="mysql">MySQL</el-radio-button>
    <el-radio-button label="postgres">PostgreSQL</el-radio-button>
    <el-radio-button label="rqlite">rqlite</el-radio-button>
  </el-radio-group>
</template>
```

This uses `form.dbType` as the model directly, and we just need to ensure `form.type = 'database'` when category switches to database.

Let me rewrite the plan with this cleaner approach.

- [ ] **Step 1: Replace the flat radio-group with two-level structure**

Replace lines 25-41:
```html
<el-form-item :label="t('conn.type')">
  <el-radio-group v-model="form.type">
    <el-radio-button label="ssh">SSH</el-radio-button>
    <el-radio-button label="telnet">Telnet</el-radio-button>
    <el-radio-button label="mosh">Mosh</el-radio-button>
    <el-radio-button label="rdp" v-if="isWindows">RDP</el-radio-button>
    <el-radio-button label="vnc">VNC</el-radio-button>
    <el-radio-button label="database">{{ t('db.database') }}</el-radio-button>
  </el-radio-group>
</el-form-item>
<el-form-item v-if="form.type === 'database'" :label="t('db.dbType')">
  <el-select v-model="form.dbType" :placeholder="t('db.dbType')">
    <el-option label="MySQL" value="mysql" />
    <el-option label="PostgreSQL" value="postgres" />
    <el-option label="rqlite" value="rqlite" />
  </el-select>
</el-form-item>
```

With:
```html
<el-form-item :label="t('conn.type')">
  <el-radio-group :model-value="category" @change="onCategoryChange">
    <el-radio-button value="terminal">{{ t('conn.categoryTerminal') }}</el-radio-button>
    <el-radio-button value="remote">{{ t('conn.categoryRemote') }}</el-radio-button>
    <el-radio-button value="database">{{ t('db.database') }}</el-radio-button>
  </el-radio-group>
</el-form-item>
<el-form-item v-if="category" label="">
  <template v-if="category === 'terminal'">
    <el-radio-group v-model="form.type">
      <el-radio-button label="ssh">SSH</el-radio-button>
      <el-radio-button label="telnet">Telnet</el-radio-button>
      <el-radio-button label="mosh">Mosh</el-radio-button>
    </el-radio-group>
  </template>
  <template v-if="category === 'remote'">
    <el-radio-group v-model="form.type">
      <el-radio-button label="rdp" v-if="isWindows">RDP</el-radio-button>
      <el-radio-button label="vnc">VNC</el-radio-button>
    </el-radio-group>
  </template>
  <template v-if="category === 'database'">
    <el-radio-group v-model="form.dbType">
      <el-radio-button label="mysql">MySQL</el-radio-button>
      <el-radio-button label="postgres">PostgreSQL</el-radio-button>
      <el-radio-button label="rqlite">rqlite</el-radio-button>
    </el-radio-group>
  </template>
</el-form-item>
```

---

### Task 3: Add category computed and change handlers in script

**Files:**
- Modify: `frontend/src/components/ConnectionForm.vue` (script section)

- [ ] **Step 1: Add category mapping utilities and computed**

Add after the `isEdit` computed (line ~155):
```typescript
const TERMINAL_TYPES = ['ssh', 'telnet', 'mosh']
const REMOTE_TYPES = ['rdp', 'vnc']

const category = computed(() => {
  if (TERMINAL_TYPES.includes(form.type)) return 'terminal'
  if (REMOTE_TYPES.includes(form.type)) return 'remote'
  if (form.type === 'database') return 'database'
  return 'terminal'
})
```

- [ ] **Step 2: Add category change handler**

Add:
```typescript
function onCategoryChange(cat: string) {
  if (cat === 'terminal') {
    form.type = 'ssh'
    if (!isEdit.value) form.port = 22
  } else if (cat === 'remote') {
    form.type = isWindows.value ? 'rdp' : 'vnc'
    if (!isEdit.value) form.port = isWindows.value ? 3389 : 5900
  } else if (cat === 'database') {
    form.type = 'database'
    form.dbType = form.dbType || 'mysql'
    if (!isEdit.value) form.port = 3306
  }
}
```

- [ ] **Step 3: Update the existing `form.type` watcher to use `TERMINAL_TYPES`**

Replace lines 218-230:
```typescript
// Auto-switch default port when changing type
watch(() => form.type, (newType) => {
  if (isEdit.value) return
  if (newType === 'rdp' && form.port === 22) form.port = 3389
  else if (newType === 'ssh' && form.port === 3389) form.port = 22
  else if (newType === 'vnc' && form.port === 22) form.port = 5900
  else if (newType === 'ssh' && form.port === 5900) form.port = 22
  else if (newType === 'telnet') form.port = 23
  else if (newType === 'mosh') form.port = 22
  else if (newType === 'database') form.port = 3306
  if (newType === 'rdp' || newType === 'vnc' || newType === 'database') {
    form.authType = 'password'
  }
})
```

With:
```typescript
// Auto-switch default port when changing type
watch(() => form.type, (newType, oldType) => {
  if (isEdit.value) return
  if (newType === 'rdp' && !REMOTE_TYPES.includes(oldType || '')) form.port = 3389
  else if (newType === 'vnc' && !REMOTE_TYPES.includes(oldType || '')) form.port = 5900
  else if (newType === 'ssh') form.port = 22
  else if (newType === 'telnet') form.port = 23
  else if (newType === 'mosh') form.port = 22
  else if (newType === 'database') form.port = 3306
  if (REMOTE_TYPES.includes(newType) || newType === 'database') {
    form.authType = 'password'
  }
})
```

Note: added `oldType` parameter and simplified RDP/VNC port switching by checking if the old type was also remote (avoid resetting port when toggling between RDP and VNC).

---

### Task 4: Verify field visibility conditions

**Files:**
- Modify: `frontend/src/components/ConnectionForm.vue` (template, existing v-if lines)

- [ ] **Step 1: Review existing v-if conditions — no changes needed**

The design spec's field visibility matrix is already satisfied by the current conditions:
- Line 48: `form.type !== 'vnc' && !(form.type === 'database' && form.dbType === 'rqlite')` — already excludes VNC and rqlite from user field ✓
- Line 51: `form.type === 'ssh' || form.type === 'mosh'` — authType only for SSH/Mosh ✓
- Line 57: Complex condition already covers all terminal types for password ✓
- Line 60: `form.authType === 'key' && (form.type === 'ssh' || form.type === 'mosh')` — key path only for SSH/Mosh with key auth ✓
- Line 63: `form.type === 'rdp'` — RDP resolution settings ✓
- Line 78: `form.type === 'database' && form.dbType !== 'rqlite'` — dbName ✓
- Line 81: `form.type === 'ssh' || form.type === 'telnet' || form.type === 'mosh'` — postLoginScript ✓

No changes needed. These conditions remain correct.

---

### Task 5: Build and verify

**Files:**
- None

- [ ] **Step 1: Build frontend**

```bash
cd frontend && npm run build
```

- [ ] **Step 2: Build full application**

```bash
cd .. && wails build -platform windows/amd64
```

- [ ] **Step 3: Run wails dev and test**

```bash
wails dev
```

Verify:
1. Open new connection dialog → see three category buttons
2. Select "终端" → SSH/Telnet/Mosh radio buttons shown
3. Select "远程桌面" → RDP/VNC shown
4. Select "数据库" → MySQL/PostgreSQL/rqlite shown
5. Port auto-updates when switching categories
6. Edit existing connection → correct category and protocol pre-selected
7. Save → type stored correctly in connection data
8. Sidebar right-click → connect label still works
