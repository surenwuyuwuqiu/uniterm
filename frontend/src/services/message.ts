import { ElMessage } from 'element-plus'

const CLOSABLE = { showClose: true, duration: 5000, offset: 50 }

export const msg = {
  success(m: string) { ElMessage.success({ message: m, ...CLOSABLE }) },
  error(m: string)   { ElMessage.error({ message: m, ...CLOSABLE }) },
  warning(m: string) { ElMessage.warning({ message: m, ...CLOSABLE }) },
  info(m: string)    { ElMessage.info({ message: m, ...CLOSABLE }) },
  // Same position as a toast but stays until closed, so a long path can be
  // read and copied. offset clears the 44px header, and no-drag lets the mouse
  // select text instead of the WKWebView grabbing it as a window drag on macOS.
  copyable(m: string, type: 'success' | 'info' | 'warning' | 'error' = 'success') {
    ElMessage({ message: m, type, showClose: true, duration: 0, offset: 56, customClass: 'msg-copyable' })
  },
}
