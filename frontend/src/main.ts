import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import { ElDialog } from 'element-plus'
import { WindowSetTitle } from '../wailsjs/runtime'
import App from './App.vue'
import './style.css'
import { useSettingsStore } from './stores/settingsStore'

WindowSetTitle('uniTerm')

// Set ElDialog draggable by default
if (ElDialog.props) {
  ElDialog.props.draggable = { type: Boolean, default: true }
}

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(ElementPlus)

const settingsStore = useSettingsStore()
await settingsStore.init()

app.mount('#app')

// Global context menu closer: broadcast to all menu components via window event
document.addEventListener('contextmenu', () => {
  window.dispatchEvent(new CustomEvent('global:close-context-menus'))
}, true)

document.addEventListener('contextmenu', (e) => {
  const target = e.target as HTMLElement
  // Read-only log-path toast: offer copy/select-all on its plain text.
  const copyable = target.closest('.msg-copyable') as HTMLElement | null
  if (copyable) {
    e.preventDefault()
    const content = (copyable.querySelector('.el-message__content') as HTMLElement) || copyable
    window.dispatchEvent(new CustomEvent('input:contextmenu', {
      detail: { x: e.clientX, y: e.clientY, target: content, readonly: true }
    }))
    return
  }
  const tag = target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || target.isContentEditable) {
    e.preventDefault()
    window.dispatchEvent(new CustomEvent('input:contextmenu', {
      detail: { x: e.clientX, y: e.clientY, target }
    }))
    return
  }
  e.preventDefault()
})
