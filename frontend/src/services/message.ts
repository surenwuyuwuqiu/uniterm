import { ElMessage } from 'element-plus'

const CLOSABLE = { showClose: true, duration: 5000, offset: 50 }

export const msg = {
  success(m: string) { ElMessage.success({ message: m, ...CLOSABLE }) },
  error(m: string)   { ElMessage.error({ message: m, ...CLOSABLE }) },
  warning(m: string) { ElMessage.warning({ message: m, ...CLOSABLE }) },
  info(m: string)    { ElMessage.info({ message: m, ...CLOSABLE }) },
  copyable(m: string, type: 'success' | 'info' | 'warning' | 'error' = 'success') {
    const safe = m.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;')
    ElMessage({
      dangerouslyUseHTMLString: true,
      message: `<input readonly value="${safe}" style="border:none;outline:none;background:transparent;font:inherit;color:inherit;cursor:text;width:630px">`,
      type,
      showClose: true,
      duration: 0,
      offset: 56,
    })
  },
}
