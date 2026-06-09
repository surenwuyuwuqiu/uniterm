export const SUPPORTED_LOCALES = [
  'zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'de', 'es', 'fr', 'ru'
] as const

export type Locale = typeof SUPPORTED_LOCALES[number]
export type Language = Locale | 'system'
export type Theme = 'dark' | 'deep-blue' | 'light' | 'system'
export type TerminalTheme = 'dark' | 'light' | 'solarized-dark' | 'solarized-light' | 'monokai'

export interface TerminalSettings {
  theme: TerminalTheme
  fontFamily: string
  fontSize: number
  selectionAction: 'none' | 'copy'
  rightClickAction: 'menu' | 'paste'
  maxHistoryLines: number
  smartCompletion: boolean
  highlightEnabled: boolean
}

export interface AIModelConfig {
  id: string
  name: string
  apiKey: string
  baseURL: string
  model: string
}

export interface AISettings {
  models: AIModelConfig[]
  activeModelId: string
}

export interface AppSettings {
  theme: Theme
  language: Language
  terminal: TerminalSettings
  ai: AISettings
}

export const DEFAULT_SETTINGS: AppSettings = {
  theme: 'dark',
  language: 'system',
  terminal: {
    theme: 'dark',
    fontFamily: 'Consolas, "Courier New", monospace',
    fontSize: 14,
    selectionAction: 'none',
    rightClickAction: 'menu',
    maxHistoryLines: 2500,
    smartCompletion: true,
    highlightEnabled: true
  },
  ai: {
    models: [
      {
        id: 'model-default',
        name: 'Default',
        apiKey: '',
        baseURL: 'https://api.openai.com/v1',
        model: 'gpt-4o'
      }
    ],
    activeModelId: 'model-default'
  }
}

export const TERMINAL_THEMES: { label: string; value: TerminalTheme }[] = [
  { label: 'Dark', value: 'dark' },
  { label: 'Light', value: 'light' },
  { label: 'Solarized Dark', value: 'solarized-dark' },
  { label: 'Solarized Light', value: 'solarized-light' },
  { label: 'Monokai', value: 'monokai' }
]

export const FONT_OPTIONS: { label: string; value: string }[] = [
  { label: 'Consolas', value: 'Consolas, "Courier New", monospace' },
  { label: 'Courier New', value: '"Courier New", Courier, monospace' },
  { label: 'Monaco', value: 'Monaco, "Courier New", monospace' },
  { label: 'Fira Code', value: '"Fira Code", monospace' },
  { label: 'JetBrains Mono', value: '"JetBrains Mono", monospace' },
  { label: 'Source Code Pro', value: '"Source Code Pro", monospace' }
]

export const SELECTION_ACTIONS: { label: string; value: TerminalSettings['selectionAction'] }[] = [
  { label: 'None', value: 'none' },
  { label: 'Copy to clipboard', value: 'copy' }
]

export const RIGHT_CLICK_ACTIONS: { label: string; value: TerminalSettings['rightClickAction'] }[] = [
  { label: 'Show context menu', value: 'menu' },
  { label: 'Paste from clipboard', value: 'paste' }
]

export const LANGUAGE_OPTIONS: { value: Locale; label: string; native: string }[] = [
  { value: 'zh-CN', label: '简体中文', native: '简体中文' },
  { value: 'zh-TW', label: '繁體中文', native: '繁體中文' },
  { value: 'en', label: 'English', native: 'English' },
  { value: 'ja', label: '日本語', native: '日本語' },
  { value: 'ko', label: '한국어', native: '한국어' },
  { value: 'de', label: 'Deutsch', native: 'Deutsch' },
  { value: 'es', label: 'Español', native: 'Español' },
  { value: 'fr', label: 'Français', native: 'Français' },
  { value: 'ru', label: 'Русский', native: 'Русский' },
]
