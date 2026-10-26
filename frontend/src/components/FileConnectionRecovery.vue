<template>
  <div v-if="error || (switched && connected)" class="file-connection-recovery" role="status">
    <span v-if="error">{{ backendErrorTextOf(error) }}</span>
    <span v-else>{{ t('sftp.usingScp') }}</span>
    <el-button v-if="canRetry" size="small" :loading="busy" @click="retry">{{ t('sftp.retryScp') }}</el-button>
    <el-button v-if="switched && connected && savedConnection && !remembered" size="small" @click="remember">
      {{ t('sftp.rememberScp') }}
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { useFileConnectionRecovery } from '../composables/useFileConnectionRecovery'
import { backendErrorTextOf } from '../utils/backendError'
import type { ConnectionConfig } from '../types/session'

const props = defineProps<{
  sessionId?: string | null
  config?: ConnectionConfig | null
  retryScp: () => Promise<unknown>
}>()
const { t, busy, error, switched, connected, canRetry, savedConnection, remembered, retry, remember } = useFileConnectionRecovery(props)
</script>

<style scoped>
.file-connection-recovery {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 10px;
  font-size: 12px;
  overflow-wrap: anywhere;
  color: var(--el-text-color-regular);
}
</style>
