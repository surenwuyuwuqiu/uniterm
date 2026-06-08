<template>
  <el-dialog v-model="visible" :title="isEdit ? t('conn.editTitle') : t('conn.newTitle')" width="500px">
    <el-form id="conn-form" :model="form" label-width="100px" @submit.prevent="onSave">
      <el-form-item :label="t('conn.name')">
        <el-input v-model="form.name" :placeholder="t('conn.namePlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('conn.group')">
        <el-select v-model="selectedGroupId" :placeholder="t('conn.noGroup')" clearable @change="onGroupSelect">
          <el-option
            v-for="g in connectionStore.groups"
            :key="g.id"
            :label="g.name"
            :value="g.id"
          />
          <el-option
            :label="t('conn.noGroup')"
            value="__none__"
          />
          <el-option
            :label="t('conn.newGroup')"
            value="__new__"
          />
        </el-select>
      </el-form-item>
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
            <el-radio-button label="spice">SPICE</el-radio-button>
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
      <el-form-item :label="t('conn.host')" required>
        <el-input v-model="form.host" :placeholder="t('conn.hostPlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('conn.port')">
        <el-input-number v-model="form.port" :min="1" :max="65535" />
      </el-form-item>
      <el-form-item v-if="form.type !== 'vnc' && form.type !== 'spice' && !(form.type === 'database' && form.dbType === 'rqlite')" :label="t('conn.user')">
        <el-input v-model="form.user" :placeholder="t('conn.userPlaceholder')" />
      </el-form-item>
      <el-form-item v-if="form.type === 'ssh' || form.type === 'mosh'" :label="t('conn.authType')">
        <el-radio-group v-model="form.authType">
          <el-radio-button label="password">{{ t('conn.password') }}</el-radio-button>
          <el-radio-button label="key">{{ t('conn.keyPath') }}</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="(form.authType === 'password' || form.type === 'rdp' || form.type === 'vnc' || form.type === 'spice' || form.type === 'database' || form.type === 'mosh' || form.type === 'telnet') && !(form.type === 'database' && form.dbType === 'rqlite')" :label="t('conn.password')">
        <el-input v-model="form.password" type="password" show-password :key="passwordInputKey" />
      </el-form-item>
      <el-form-item v-if="form.authType === 'key' && (form.type === 'ssh' || form.type === 'mosh')" :label="t('conn.keyPath')">
        <el-input v-model="form.keyPath" :placeholder="t('conn.keyPathPlaceholder')" />
      </el-form-item>
      <template v-if="form.type === 'rdp'">
        <el-form-item :label="t('rdp.resolution')">
          <el-select v-model="rdpResolution" placeholder="1280×720">
            <el-option
              v-for="r in rdpResolutions"
              :key="r.label"
              :label="r.label"
              :value="r.label"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('conn.rdpSmartSizing')">
          <el-switch v-model="form.rdpSmartSizing" />
        </el-form-item>
      </template>
      <el-form-item v-if="form.type === 'database' && form.dbType !== 'rqlite'" :label="t('db.databases')">
        <el-input v-model="form.dbName" :placeholder="t('db.databases')" />
      </el-form-item>
      <el-form-item v-if="form.type === 'ssh' || form.type === 'telnet' || form.type === 'mosh'" :label="t('conn.postLoginScript')">
        <el-input
          v-model="form.postLoginScript"
          type="textarea"
          :rows="3"
          :placeholder="t('conn.postLoginScriptPlaceholder')"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('conn.cancel') }}</el-button>
      <el-button @click="onSave">{{ t('conn.saveOnly') }}</el-button>
      <el-button type="primary" @click="onConnect">{{ isEdit ? t('conn.saveConnect') : t('conn.connect') }}</el-button>
    </template>
  </el-dialog>

  <!-- New group dialog -->
  <el-dialog v-model="showNewGroupDialog" :title="t('conn.newGroupTitle')" width="360px">
    <el-form @submit.prevent="confirmNewGroup">
      <el-form-item :label="t('conn.groupName')">
        <el-input
          v-model="newGroupName"
          :placeholder="t('conn.groupNamePlaceholder')"
          @keyup.enter="confirmNewGroup"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="showNewGroupDialog = false">{{ t('conn.cancel') }}</el-button>
      <el-button type="primary" @click="confirmNewGroup">{{ t('conn.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, computed, watch, ref, onMounted } from 'vue'
import { useConnectionStore } from '../stores/connectionStore'
import { useI18n } from '../i18n'
import type { ConnectionConfig } from '../types/session'
import { GetPlatform } from '../../wailsjs/go/main/App'

const { t } = useI18n()
const connectionStore = useConnectionStore()

const isWindows = ref(true)
const passwordInputKey = ref(0)

onMounted(async () => {
  try { isWindows.value = (await GetPlatform()) === 'windows' } catch (_) {}
})

const props = defineProps<{
  modelValue: boolean
  editConfig?: ConnectionConfig
  defaultGroupId?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [config: ConnectionConfig]
  connect: [config: ConnectionConfig]
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

watch(visible, (val) => {
  if (val) {
    passwordInputKey.value++
  }
})

const isEdit = computed(() => !!props.editConfig?.id)

const TERMINAL_TYPES = ['ssh', 'telnet', 'mosh']
const REMOTE_TYPES = ['rdp', 'vnc', 'spice']

const category = computed(() => {
  if (TERMINAL_TYPES.includes(form.type)) return 'terminal'
  if (REMOTE_TYPES.includes(form.type)) return 'remote'
  if (form.type === 'database') return 'database'
  return 'terminal'
})

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

const form = reactive<ConnectionConfig>({
  id: '',
  name: '',
  type: 'ssh',
  host: '',
  port: 22,
  user: '',
  authType: 'password',
  password: '',
  keyPath: '',
  groupId: undefined,
  rdpFixedWidth: undefined,
  rdpFixedHeight: undefined,
  rdpSmartSizing: true,
  dbType: '',
  dbName: '',
  postLoginScript: '',
})

const rdpResolutions = [
  { label: '800 × 600 (SVGA)', w: 800, h: 600 },
  { label: '1024 × 768 (XGA)', w: 1024, h: 768 },
  { label: '1280 × 720 (HD)', w: 1280, h: 720 },
  { label: '1680 × 1050 (WSXGA+)', w: 1680, h: 1050 },
  { label: '1600 × 1200 (UXGA)', w: 1600, h: 1200 },
  { label: '1920 × 1080 (Full HD)', w: 1920, h: 1080 },
  { label: '2560 × 1440 (QHD)', w: 2560, h: 1440 },
]

const rdpResolution = ref('1280 × 720 (HD)')

const selectedGroupId = ref<string | undefined>(undefined)

// New group dialog
const showNewGroupDialog = ref(false)
const newGroupName = ref('')

watch(() => props.editConfig, (config) => {
  if (config) {
    Object.assign(form, { ...config })
    selectedGroupId.value = config.groupId || undefined
    // Sync resolution dropdown to the config's fixed size
    const match = rdpResolutions.find(r => r.w === config.rdpFixedWidth && r.h === config.rdpFixedHeight)
    if (match) rdpResolution.value = match.label
  } else {
    resetForm()
    if (props.defaultGroupId) {
      selectedGroupId.value = props.defaultGroupId
      form.groupId = props.defaultGroupId
    }
  }
}, { immediate: true })

watch(() => props.defaultGroupId, (gid) => {
  if (!props.editConfig && gid) {
    selectedGroupId.value = gid
    form.groupId = gid
  }
})

// Auto-switch default port when changing type
watch(() => form.type, (newType, oldType) => {
  if (isEdit.value) return
  if (newType === 'rdp' && !REMOTE_TYPES.includes(oldType || '')) form.port = 3389
  else if (newType === 'vnc' && !REMOTE_TYPES.includes(oldType || '')) form.port = 5900
  else if (newType === 'spice' && !REMOTE_TYPES.includes(oldType || '')) form.port = 5900
  else if (newType === 'ssh') form.port = 22
  else if (newType === 'telnet') form.port = 23
  else if (newType === 'mosh') form.port = 22
  else if (newType === 'database') form.port = 3306
  if (REMOTE_TYPES.includes(newType) || newType === 'database') {
    form.authType = 'password'
  }
})

// Auto-switch default port when changing database type
watch(() => form.dbType, (newType) => {
  if (isEdit.value) return
  if (newType === 'mysql') form.port = 3306
  else if (newType === 'postgres') form.port = 5432
  else if (newType === 'rqlite') form.port = 4001
})

// Sync resolution picker to form fields
watch(rdpResolution, (val) => {
  const found = rdpResolutions.find(r => r.label === val)
  if (found) {
    form.rdpFixedWidth = found.w
    form.rdpFixedHeight = found.h
  }
})

function resetForm() {
  form.id = ''
  form.name = ''
  form.type = 'ssh'
  form.host = ''
  form.port = 22
  form.user = ''
  form.authType = 'password'
  form.password = ''
  form.keyPath = ''
  form.groupId = undefined
  form.rdpFixedWidth = undefined
  form.rdpFixedHeight = undefined
  form.rdpSmartSizing = true
  form.dbType = ''
  form.dbName = ''
  form.postLoginScript = ''
  rdpResolution.value = '1280 × 720 (HD)'
  selectedGroupId.value = undefined
}

function onGroupSelect(value: string | undefined) {
  if (value === '__new__') {
    showNewGroupDialog.value = true
    newGroupName.value = ''
    selectedGroupId.value = form.groupId || undefined
    return
  }
  if (value === '__none__') {
    form.groupId = undefined
    selectedGroupId.value = undefined
    return
  }
  form.groupId = value
  selectedGroupId.value = value
}

async function confirmNewGroup() {
  const name = newGroupName.value.trim()
  if (!name) {
    return
  }
  if (connectionStore.groups.some(g => g.name === name)) {
    return
  }
  const group = await connectionStore.addGroup(name)
  form.groupId = group.id
  selectedGroupId.value = group.id
  showNewGroupDialog.value = false
}

function generateUniqueName(name: string): string {
  if (!connectionStore.connections.some(c => c.name === name)) {
    return name
  }
  let idx = 1
  while (connectionStore.connections.some(c => c.name === `${name} (${idx})`)) {
    idx++
  }
  return `${name} (${idx})`
}

function normalizeForm(): ConnectionConfig {
  const normalized = { ...form }
  if (!normalized.host.trim()) {
    throw new Error(t('conn.hostRequired'))
  }
  if (!normalized.name.trim()) {
    normalized.name = generateUniqueName(normalized.host.trim())
  }
  return normalized
}

function onSave() {
  try {
    const config = normalizeForm()
    emit('save', config)
    visible.value = false
    if (!props.editConfig) {
      resetForm()
    }
  } catch (e: any) {
    // Host empty, silently return
  }
}

function onConnect() {
  try {
    const config = normalizeForm()
    emit('connect', config)
    visible.value = false
    if (!props.editConfig) {
      resetForm()
    }
  } catch (e: any) {
    // Host empty
  }
}
</script>
