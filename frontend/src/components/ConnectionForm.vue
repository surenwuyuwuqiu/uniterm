<template>
  <el-dialog v-model="visible" :title="isEdit ? t('conn.editTitle') : t('conn.newTitle')" width="500px">
    <el-form id="conn-form" :model="form" label-width="100px" @submit.prevent="onConnect">
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
        <el-radio-group v-model="form.type">
          <el-radio-button label="ssh">SSH</el-radio-button>
          <el-radio-button label="rdp">RDP</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item :label="t('conn.host')" required>
        <el-input v-model="form.host" :placeholder="t('conn.hostPlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('conn.port')">
        <el-input-number v-model="form.port" :min="1" :max="65535" />
      </el-form-item>
      <el-form-item :label="t('conn.user')">
        <el-input v-model="form.user" :placeholder="t('conn.userPlaceholder')" />
      </el-form-item>
      <el-form-item v-if="form.type !== 'rdp'" :label="t('conn.authType')">
        <el-radio-group v-model="form.authType">
          <el-radio-button label="password">{{ t('conn.password') }}</el-radio-button>
          <el-radio-button label="key">{{ t('conn.keyPath') }}</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="form.authType === 'password' || form.type === 'rdp'" :label="t('conn.password')">
        <el-input v-model="form.password" type="password" show-password />
      </el-form-item>
      <el-form-item v-if="form.authType === 'key' && form.type !== 'rdp'" :label="t('conn.keyPath')">
        <el-input v-model="form.keyPath" :placeholder="t('conn.keyPathPlaceholder')" />
      </el-form-item>
      <template v-if="form.type === 'rdp'">
        <el-form-item :label="t('conn.rdpSizeMode')">
          <el-radio-group v-model="form.rdpSizeMode">
            <el-radio-button label="follow">{{ t('conn.rdpFollowWindow') }}</el-radio-button>
            <el-radio-button label="fixed">{{ t('conn.rdpFixedSize') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.rdpSizeMode === 'fixed'" :label="t('rdp.resolution')">
          <el-select v-model="rdpResolution" placeholder="1280×720">
            <el-option
              v-for="r in rdpResolutions"
              :key="r.label"
              :label="r.label"
              :value="r.label"
            />
          </el-select>
        </el-form-item>
      </template>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('conn.cancel') }}</el-button>
      <el-button @click="onSave">{{ t('conn.saveOnly') }}</el-button>
      <el-button type="primary" native-type="submit" form="conn-form">{{ isEdit ? t('conn.saveConnect') : t('conn.connect') }}</el-button>
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
import { reactive, computed, watch, ref } from 'vue'
import { useConnectionStore } from '../stores/connectionStore'
import { useI18n } from '../i18n'
import type { ConnectionConfig } from '../types/session'

const { t } = useI18n()
const connectionStore = useConnectionStore()

const props = defineProps<{
  modelValue: boolean
  editConfig?: ConnectionConfig
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

const isEdit = computed(() => !!props.editConfig?.id)

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
  rdpSizeMode: 'follow',
  rdpFixedWidth: undefined,
  rdpFixedHeight: undefined
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
  }
}, { immediate: true })

// Auto-switch default port when changing type
watch(() => form.type, (newType) => {
  if (newType === 'rdp' && form.port === 22) form.port = 3389
  else if (newType === 'ssh' && form.port === 3389) form.port = 22
  if (newType === 'rdp') {
    form.authType = 'password'
    form.rdpSizeMode = form.rdpSizeMode || 'follow'
  }
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
  form.rdpSizeMode = 'follow'
  form.rdpFixedWidth = undefined
  form.rdpFixedHeight = undefined
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
