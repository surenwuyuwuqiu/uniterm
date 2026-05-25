export type SessionStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

export interface ConnectionGroup {
  id: string
  name: string
}

export interface ConnectionConfig {
  id: string
  name: string
  type: 'ssh' | 'rdp' | 'vnc' | 'database'
  host: string
  port: number
  user: string
  authType: 'password' | 'key' | 'agent'
  password?: string
  keyPath?: string
  groupId?: string
  // RDP-specific
  rdpFixedWidth?: number
  rdpFixedHeight?: number
  rdpSmartSizing?: boolean
  // Local terminal shell path
  shellPath?: string
  dbType?: string   // "mysql", "postgres", "rqlite"
  dbName?: string   // default database name
}

export interface SessionInfo {
  id: string
  type: string
  title: string
  status: SessionStatus
}

export interface Tab {
  id: string
  sessionId: string
  title: string
  type: 'ssh' | 'settings'
  groupId?: string
  config?: ConnectionConfig
  aiLocked?: boolean
}

export interface SplitNode {
  id: string
  direction: 'horizontal' | 'vertical' | null
  children: SplitNode[]
  tabGroupId?: string
  ratio: number
}
