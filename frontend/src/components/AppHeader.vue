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
    <button class="header-btn" @click="emit('toggle-sidebar')" :title="t('header.connections')">
      <el-icon><Network :size="14" /></el-icon>
    </button>

    <!-- New connection dropdown (+ icon, after connections) -->
    <el-dropdown
      v-if="settingsStore.availableShells.length > 0"
      trigger="click"
      @command="onNewCommand"
      @visible-change="onShellDropdownVisibleChange"
    >
      <button class="header-btn" :title="t('header.newConnection')">
        <el-icon><Plus :size="14" /></el-icon>
      </button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item command="new-connection">{{ t('header.newConnection') }}</el-dropdown-item>
          <div
            v-if="settingsStore.availableShells.length > 0"
            ref="submenuTriggerRef"
            class="submenu-wrapper"
            @mouseenter="onShellTriggerEnter"
            @mouseleave="onShellTriggerLeave"
          >
            <el-dropdown-item class="submenu-trigger">
              {{ t('header.newLocalTerminal') }} <el-icon class="submenu-arrow"><ChevronRight :size="12" /></el-icon>
            </el-dropdown-item>
          </div>
          <Teleport to="body">
            <div
              v-show="showShellSubmenu"
              class="shell-submenu"
              :style="shellSubmenuStyle"
              @mouseenter="showShellSubmenu = true"
              @mouseleave="showShellSubmenu = false"
            >
              <div
                v-for="sh in settingsStore.availableShells"
                :key="sh"
                class="shell-item"
                @click="onShellSelect(sh)"
              >
                {{ getShellLabel(sh) }}
              </div>
            </div>
          </Teleport>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
    <button v-else class="header-btn" @click="emit('new-connection')" :title="t('header.newConnection')">
      <el-icon><Plus :size="14" /></el-icon>
    </button>

    <!-- Tabs list -->
    <div class="header-tabs" :class="{ 'tabs-centered': platform === 'darwin' }">
      <TabsList
        @close-tab="(id: string) => emit('close-tab', id)"
        @toggle-ai-lock="(panelId: string) => emit('toggle-ai-lock', panelId)"
        @tab-dragstart="(e: DragEvent, tabId: string) => emit('tab-dragstart', e, tabId)"
      />
    </div>

    <!-- AI button (icon only) -->
    <button class="header-btn accent ai-btn" @click="emit('toggle-ai')" :title="t('header.ai')">
      AI
    </button>

    <!-- Settings button (icon only, rightmost) -->
    <button class="header-btn" @click="emit('open-settings')" :title="t('header.settings')">
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

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Plus, Network, Settings, ChevronRight } from '@lucide/vue'
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

function onShellSelect(sh: string) {
  showShellSubmenu.value = false
  emit('new-local-terminal-with-shell', sh)
}

function getShellLabel(path: string): string {
  const lower = path.toLowerCase()
  if (lower.includes('pwsh')) return 'PowerShell'
  if (lower.includes('powershell')) return 'Windows PowerShell'
  if (lower.includes('bash')) return 'Git Bash'
  if (lower.includes('cmd')) return 'Command Prompt'
  return path.split(/[\\/]/).pop() || path
}

const showShellSubmenu = ref(false)
const submenuTriggerRef = ref<HTMLElement | null>(null)
const shellSubmenuStyle = ref<Record<string, string>>({})

function onShellTriggerEnter() {
  showShellSubmenu.value = true
  nextTick(() => {
    const el = submenuTriggerRef.value
    if (!el) return
    const rect = el.getBoundingClientRect()
    shellSubmenuStyle.value = {
      position: 'fixed',
      left: rect.right + 4 + 'px',
      top: rect.top + 'px',
    }
  })
}

function onShellTriggerLeave() {
  showShellSubmenu.value = false
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
</script>

<style scoped>
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
  justify-content: flex-start;
}

.header-tabs.tabs-centered {
  justify-content: center;
}

.header-btn {
  display: flex;
  align-items: center;
  justify-content: center;
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

.ai-btn {
  font-weight: 700;
  font-size: 12px;
  letter-spacing: 0.5px;
  min-width: 28px;
}

.header-btn .el-icon {
  font-size: 14px;
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

/* Allow submenu to overflow the dropdown menu */
:deep(.el-dropdown-menu) {
  overflow: visible !important;
}
:deep(.el-dropdown-menu__item) {
  position: static;
}
:deep(.el-scrollbar),
:deep(.el-scrollbar__wrap),
:deep(.el-scrollbar__view) {
  overflow: visible !important;
  max-height: none !important;
}

.submenu-trigger {
  justify-content: space-between;
}
.submenu-arrow {
  margin-left: 16px;
  font-size: 10px;
  color: var(--text-muted);
}

.shell-submenu {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  padding: 4px;
  min-width: 160px;
  z-index: 10000;
}

.shell-item {
  padding: 8px 16px;
  font-size: 12px;
  color: var(--text-secondary);
  cursor: pointer;
  border-radius: var(--radius-sm);
  white-space: nowrap;
}
.shell-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
</style>
