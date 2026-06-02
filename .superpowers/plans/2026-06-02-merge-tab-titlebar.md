# Merge Tab Bar into Titlebar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merge the tag bar into the titlebar into a single line, saving 84px → 40-44px vertical space. All buttons become icon-only, new connection + local terminal merged into one `+` dropdown.

**Architecture:** Create a new `TabsList.vue` component extracting tabs rendering logic from `TabBar.vue`. `AppHeader.vue` imports `TabsList` directly and renders it between buttons and drag area. `App.vue` stops rendering `<TabBar />`. All button text removed, brand text removed.

**Tech Stack:** Vue 3, Element Plus, Lucide icons, TypeScript

---

### Task 1: Create TabsList.vue component

**Files:**
- Create: `frontend/src/components/TabsList.vue`
- Reference: `frontend/src/components/TabBar.vue` (tabs rendering parts)

Extract the tabs rendering logic from TabBar.vue into a standalone component. This contains only the `.tabs-list` + `.tab-more` UI and drag-drop orchestration. Tab close logic stays in TabBar for now (we don't use it from AppHeader).

- [ ] **Step 1: Create TabsList.vue with template and style**

```vue
<template>
  <div class="tabs-list" ref="tabsListRef" @wheel="onWheel">
    <template v-for="(tab, index) in tabs" :key="tab.id">
      <div
        v-if="(dragOverTabIndex === index && !dragOverInsertAfter) || (dragOverTabIndex === index - 1 && dragOverInsertAfter)"
        class="tab-drop-indicator"
      ></div>

      <TabItem
        v-if="tab.type === 'terminal' || tab.type === 'settings' || tab.type === 'sftp' || tab.type === 'rdp' || tab.type === 'vnc' || tab.type === 'database' || tab.type === 'monitor'"
        :tab="tab"
        :is-active="tab.id === activeTabId"
        @activate="setActiveTab"
        @close="(id: string) => $emit('close-tab', id)"
        @toggle-ai-lock="(panelId: string) => $emit('toggle-ai-lock', panelId)"
        @dragstart="(e: DragEvent, tabId: string) => $emit('tab-dragstart', e, tabId)"
        @dragover.prevent="(e: DragEvent) => onTabDragOver(e, index)"
        @dragleave="onTabDragLeave"
        @drop="(e: DragEvent) => onTabDrop(e, tab.id, index)"
      />
      <WorkspaceTabItem
        v-else-if="tab.type === 'workspace'"
        :tab="tab"
        :is-active="tab.id === activeTabId"
        @activate="setActiveTab"
        @close="(id: string) => $emit('close-tab', id)"
        @dragstart="(e: DragEvent, tabId: string) => $emit('tab-dragstart', e, tabId)"
        @dragover.prevent="(e: DragEvent) => onTabDragOver(e, index)"
        @dragleave="onTabDragLeave"
        @drop="(e: DragEvent) => onTabDrop(e, tab.id, index)"
      />
    </template>
    <div
      v-if="dragOverTabIndex === tabs.length - 1 && dragOverInsertAfter"
      class="tab-drop-indicator"
    ></div>
  </div>
  <div class="tab-more" v-if="tabs.length > 0">
    <el-dropdown trigger="click" @command="setActiveTab" @visible-change="onMoreDropdownVisibleChange">
      <span class="tab-more-btn" :title="t('tab.more')">
        <el-icon class="tab-more-icon"><MoreHorizontal :size="14" /></el-icon>
      </span>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item
            v-for="tab in tabs"
            :key="tab.id"
            :command="tab.id"
            :class="{ 'is-active': tab.id === activeTabId }"
          >
            {{ tab.name }}
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { MoreHorizontal } from '@lucide/vue'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'
import { useI18n } from '../i18n'
import TabItem from './TabItem.vue'
import WorkspaceTabItem from './WorkspaceTabItem.vue'

const tabStore = useTabStore()
const panelStore = usePanelStore()
const { t } = useI18n()
const tabs = computed(() => tabStore.tabs)
const activeTabId = computed(() => tabStore.activeTabId)

const dragOverTabIndex = ref<number | null>(null)
const dragOverInsertAfter = ref(false)

const tabsListRef = ref<HTMLElement | null>(null)

defineEmits(['close-tab', 'toggle-ai-lock', 'tab-dragstart'])

function onWheel(e: WheelEvent) {
  if (!tabsListRef.value) return
  tabsListRef.value.scrollLeft += e.deltaY
}

function onMoreDropdownVisibleChange(visible: boolean) {
  if (visible) {
    window.dispatchEvent(new CustomEvent('rdp:overlay-push'))
  } else {
    window.dispatchEvent(new CustomEvent('rdp:overlay-pop'))
  }
}

function setActiveTab(id: string) {
  tabStore.setActiveTab(id)
  scrollToTab(id)
}

function scrollToTab(tabId: string) {
  if (!tabsListRef.value) return
  const el = tabsListRef.value.querySelector(`[data-tab-id="${tabId}"]`) as HTMLElement | null
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' })
  }
}

function onTabDragOver(e: DragEvent, index: number) {
  const hasPanel = e.dataTransfer?.types.includes('application/panel-id')
  const hasTab = e.dataTransfer?.types.includes('application/tab-id')
  if (!hasPanel && !hasTab) return

  const el = e.currentTarget as HTMLElement
  const rect = el.getBoundingClientRect()
  dragOverTabIndex.value = index
  dragOverInsertAfter.value = e.clientX >= rect.left + rect.width / 2
  e.dataTransfer!.dropEffect = 'move'
}

function onTabDragLeave(_e: DragEvent) {
  // Reset handled by onTabDragOver of adjacent tab
}

function onTabDrop(e: DragEvent, targetTabId: string, index: number) {
  e.stopPropagation()

  const insertAfter = dragOverInsertAfter.value
  dragOverTabIndex.value = null
  dragOverInsertAfter.value = false

  const draggedTabId = e.dataTransfer?.getData('application/tab-id')
  const draggedPanelId = e.dataTransfer?.getData('application/panel-id')
  const sourceTabId = e.dataTransfer?.getData('application/source-tab-id')

  if (draggedTabId && !draggedPanelId) {
    if (draggedTabId === targetTabId) return
    const fromIdx = tabs.value.findIndex(t => t.id === draggedTabId)
    if (fromIdx === -1) return
    let toIdx = index + (insertAfter ? 1 : 0)
    if (toIdx > fromIdx) toIdx--
    if (fromIdx !== toIdx) {
      tabStore.moveTab(fromIdx, toIdx)
    }
    return
  }

  if (draggedPanelId) {
    const panel = panelStore.getPanel(draggedPanelId)
    if (!panel) return

    if (sourceTabId) {
      tabStore.removePanelFromWorkspaceTab(sourceTabId, draggedPanelId)
    }

    const tab = tabStore.createTerminalTab(panel.title, draggedPanelId)
    panelStore.movePanelToTab(draggedPanelId, tab.id)

    const targetIdx = index + (insertAfter ? 1 : 0)
    const currentIdx = tabs.value.findIndex(t => t.id === tab.id)
    if (currentIdx !== targetIdx) {
      tabStore.moveTab(currentIdx, targetIdx)
    }
  }
}
</script>

<style scoped>
.tabs-list {
  display: flex;
  flex: 1;
  overflow-x: auto;
  overflow-y: hidden;
  align-items: stretch;
  scrollbar-width: none;
  min-width: 0;
}
.tabs-list::-webkit-scrollbar {
  display: none;
}

.tab-more {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  padding: 0 4px;
}
.tab-more-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-muted);
  letter-spacing: 1px;
  user-select: none;
  transition: all 0.15s;
}
.tab-more-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.tab-drop-indicator {
  width: 2px;
  min-width: 2px;
  align-self: stretch;
  background: var(--accent);
  opacity: 0.8;
  margin: 4px 0;
  border-radius: 1px;
  flex-shrink: 0;
}
</style>
```

- [ ] **Step 2: Verify TabsList.vue has no TypeScript errors**

Run: `cd frontend && npx vue-tsc --noEmit --skipLibCheck src/components/TabsList.vue 2>&1 || true`

---

### Task 2: Rewrite AppHeader.vue — layout, buttons, and embed TabsList

**Files:**
- Modify: `frontend/src/components/AppHeader.vue`

- [ ] **Step 1: Replace the entire template**

```vue
<template>
  <div
    class="app-header"
    @dblclick="onDblClick"
  >
    <!-- macOS: window controls left -->
    <WindowControls
      v-if="platform === 'darwin'"
      :platform="platform"
      :is-maximised="isMaximised"
      @minimise="onMinimise"
      @maximise="onMaximise"
      @close="onClose"
    />

    <!-- Connections button (icon only, leftmost) -->
    <button class="header-btn icon-only" @click="$emit('toggle-sidebar')" :title="t('header.connections')">
      <el-icon><Network :size="14" /></el-icon>
    </button>

    <!-- Tabs list (fills center on Windows, centered on macOS) -->
    <div class="header-tabs" :class="{ 'tabs-centered': platform === 'darwin' }">
      <TabsList
        @close-tab="(id: string) => $emit('close-tab', id)"
        @toggle-ai-lock="(panelId: string) => $emit('toggle-ai-lock', panelId)"
        @tab-dragstart="(e: DragEvent, tabId: string) => $emit('tab-dragstart', e, tabId)"
      />
    </div>

    <!-- New connection dropdown (+ icon, after tabs) -->
    <el-dropdown
      trigger="click"
      @command="onNewCommand"
      @visible-change="onShellDropdownVisibleChange"
    >
      <button class="header-btn icon-only" :title="t('header.newConnection')">
        <el-icon><Plus :size="14" /></el-icon>
      </button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item command="new-connection">{{ t('header.newConnection') }}</el-dropdown-item>
          <el-dropdown-item
            v-if="settingsStore.availableShells.length > 0"
            divided
            disabled
            class="dropdown-section-label"
          >
            {{ t('header.localTerminal') }}
          </el-dropdown-item>
          <el-dropdown-item
            v-for="sh in settingsStore.availableShells"
            :key="sh"
            :command="'shell:' + sh"
          >
            {{ getShellLabel(sh) }}
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>

    <!-- Spacer for drag region (middle area) -->
    <div class="header-drag-spacer"></div>

    <!-- AI button (icon only) -->
    <button class="header-btn accent icon-only" @click="$emit('toggle-ai')" :title="t('header.ai')">
      <el-icon><MessageCircleMore :size="14" /></el-icon>
    </button>

    <!-- Settings button (icon only, rightmost before window controls) -->
    <button class="header-btn icon-only" @click="$emit('open-settings')" :title="t('header.settings')">
      <el-icon><Settings :size="14" /></el-icon>
    </button>

    <!-- Windows/Linux: window controls right -->
    <WindowControls
      v-if="platform !== 'darwin'"
      :platform="platform"
      :is-maximised="isMaximised"
      @minimise="onMinimise"
      @maximise="onMaximise"
      @close="onClose"
    />
  </div>
</template>
```

- [ ] **Step 2: Update script — imports, emits, new command handler**

Replace the script section:

```typescript
import { ref, onMounted, onUnmounted } from 'vue'
import { Plus, MessageCircleMore, Network, Settings } from '@lucide/vue'
import { useI18n } from '../i18n'
import { useSettingsStore } from '../stores/settingsStore'
import WindowControls from './WindowControls.vue'
import TabsList from './TabsList.vue'
import {
  Environment,
  WindowMinimise,
  WindowToggleMaximise,
  WindowIsMaximised,
  Quit,
} from '../../wailsjs/runtime'

const { t } = useI18n()
const settingsStore = useSettingsStore()

defineEmits(['new-connection', 'new-local-terminal-with-shell', 'toggle-ai', 'toggle-sidebar', 'open-settings', 'close-tab', 'toggle-ai-lock', 'tab-dragstart'])

function onNewCommand(cmd: string) {
  if (cmd === 'new-connection') {
    // emit new-connection via a wrapper
    const emit = defineEmits ? undefined : undefined
    // Use the component instance to emit
  } else if (cmd.startsWith('shell:')) {
    const path = cmd.slice(6)
    // emit new-local-terminal-with-shell
  }
}
```

Wait — `defineEmits` returns a function we need to call. Let me fix this:

```typescript
import { ref, onMounted, onUnmounted } from 'vue'
import { Plus, MessageCircleMore, Network, Settings } from '@lucide/vue'
import { useI18n } from '../i18n'
import { useSettingsStore } from '../stores/settingsStore'
import WindowControls from './WindowControls.vue'
import TabsList from './TabsList.vue'
import {
  Environment,
  WindowMinimise,
  WindowToggleMaximise,
  WindowIsMaximised,
  Quit,
} from '../../wailsjs/runtime'

const { t } = useI18n()
const settingsStore = useSettingsStore()

const emit = defineEmits<{
  'new-connection': []
  'new-local-terminal-with-shell': [path: string]
  'toggle-ai': []
  'toggle-sidebar': []
  'open-settings': []
  'close-tab': [id: string]
  'toggle-ai-lock': [panelId: string]
  'tab-dragstart': [e: DragEvent, tabId: string]
}>()

function onNewCommand(cmd: string) {
  if (cmd === 'new-connection') {
    emit('new-connection')
  } else if (cmd.startsWith('shell:')) {
    emit('new-local-terminal-with-shell', cmd.slice(6))
  }
}

function getShellLabel(path: string): string {
  const lower = path.toLowerCase()
  if (lower.includes('pwsh')) return 'PowerShell'
  if (lower.includes('powershell')) return 'Windows PowerShell'
  if (lower.includes('bash')) return 'Git Bash'
  if (lower.includes('cmd')) return 'Command Prompt'
  return path.split(/[\\/]/).pop() || path
}

const platform = ref<'windows' | 'darwin' | 'linux'>('windows')
const isMaximised = ref(false)

async function updateMaximisedState() {
  try {
    isMaximised.value = await WindowIsMaximised()
  } catch {
    // ignore
  }
}

function onMinimise() {
  WindowMinimise()
}

async function onMaximise() {
  WindowToggleMaximise()
  setTimeout(updateMaximisedState, 100)
}

function onClose() {
  Quit()
}

function onShellDropdownVisibleChange(visible: boolean) {
  if (visible) {
    window.dispatchEvent(new CustomEvent('rdp:overlay-push'))
  } else {
    window.dispatchEvent(new CustomEvent('rdp:overlay-pop'))
  }
}

function onDblClick(e: MouseEvent) {
  if (platform.value === 'darwin') return
  const target = e.target as HTMLElement
  if (target.closest('button') || target.closest('.window-controls')) return
  onMaximise()
}

function onWindowResize() {
  updateMaximisedState()
}

onMounted(async () => {
  try {
    const env = await Environment()
    const p = env.platform.toLowerCase()
    if (p === 'darwin') platform.value = 'darwin'
    else if (p === 'linux') platform.value = 'linux'
    else platform.value = 'windows'
  } catch {
    platform.value = 'windows'
  }
  updateMaximisedState()
  window.addEventListener('resize', onWindowResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', onWindowResize)
})
```

- [ ] **Step 3: Replace the style block**

```css
.app-header {
  display: flex;
  align-items: center;
  height: 44px;
  padding: 0 8px;
  gap: 6px;
  background: var(--bg-elevated);
  flex-shrink: 0;
  position: relative;
  z-index: 10;
  --wails-draggable: drag;
}

.app-header::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    var(--accent-subtle) 20%,
    var(--accent-glow) 50%,
    var(--accent-subtle) 80%,
    transparent 100%
  );
}

.header-tabs {
  display: flex;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  --wails-draggable: no-drag;
}

.header-tabs.tabs-centered {
  justify-content: center;
}

.header-drag-spacer {
  flex: 1;
  min-width: 0;
}

.header-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 5px 8px;
  font-family: var(--font-ui);
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
  flex-shrink: 0;
  --wails-draggable: no-drag;
}

.header-btn.icon-only {
  padding: 5px 8px;
}

.header-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.header-btn.accent {
  background: linear-gradient(135deg, var(--accent-dim), var(--accent));
  color: #fff;
  box-shadow: 0 0 0 1px var(--accent-glow), 0 2px 8px var(--accent-glow);
}

.header-btn.accent:hover {
  background: linear-gradient(135deg, var(--accent), var(--accent-dim));
  box-shadow: 0 0 0 1px var(--accent-glow), 0 4px 16px var(--accent-glow);
  transform: translateY(-1px);
}

.header-btn .el-icon {
  font-size: 14px;
}

.dropdown-section-label {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  pointer-events: none;
}

[data-theme="light"] .app-header::after {
  background: linear-gradient(
    90deg,
    transparent 0%,
    var(--accent-subtle) 20%,
    var(--accent-glow) 50%,
    var(--accent-subtle) 80%,
    transparent 100%
  );
}

.app-header :deep(.window-controls) {
  --wails-draggable: no-drag;
}
```

Note: On macOS, `.header-drag-spacer` should be hidden since tabs are centered and the whole header serves as drag area. On Windows, the spacer provides the middle drag region between tabs and settings/AI.

Actually, let me simplify: remove `.header-drag-spacer` on macOS and keep it on Windows only:

```css
/* On macOS, tabs are centered — no need for spacer */
/* On Windows, spacer fills between tabs and right actions */
.header-drag-spacer {
  flex: 1;
  min-width: 0;
}

/* Hide spacer on macOS — tabs-centered already fills center */
@media not all {
  /* We'll use a class instead */
}
```

Better approach: use `v-if="platform !== 'darwin'"` on the spacer div.

---

### Task 3: Update App.vue — remove TabBar, wire new events

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Remove `<TabBar />` from template**

Replace line 14:
```html
        <TabBar />
```
With nothing (remove the line).

- [ ] **Step 2: Add new event handlers to AppHeader**

Replace the AppHeader tag (lines 3-10) with:

```html
    <AppHeader
      @new-connection="showConnectionForm = true"
      @new-local-terminal-with-shell="createLocalTerminalWithShell"
      @toggle-ai="aiStore.toggle"
      @toggle-sidebar="sidebarVisible = !sidebarVisible"
      @open-settings="openSettings"
      @close-tab="closeTab"
      @toggle-ai-lock="onToggleAiLock"
      @tab-dragstart="onTabDragStart"
    />
```

- [ ] **Step 3: Remove TabBar import**

Remove line 90:
```typescript
import TabBar from './components/TabBar.vue'
```

- [ ] **Step 4: Add `onToggleAiLock` method if not exists**

Find the `onToggleAiLock` function or add it. Check if it exists in App.vue. If not, add:

```typescript
function onToggleAiLock(panelId: string) {
  if (tabStore.aiLockedPanelId === panelId) {
    tabStore.setAILockedPanel(null)
  } else {
    tabStore.setAILockedPanel(panelId)
  }
}
```

And `onTabDragStart`:
```typescript
function onTabDragStart(_e: DragEvent, _tabId: string) {
  // No-op: data is set in TabItem/WorkspaceTabItem
}
```

---

### Task 4: Build and verify

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
1. Header shows tabs integrated in a single row
2. All buttons are icon-only
3. `+` button opens dropdown with "New Connection" and local terminal options
4. Clicking `+` icon directly opens new connection form (when no shells available) or shows dropdown
5. Tabs scroll, drag-reorder, and overflow menu work
6. macOS: window controls left, tabs centered
7. Windows: tabs left after connections, window controls right
8. AI and Settings icons work
9. Vertical space reduced from ~84px to ~44px
