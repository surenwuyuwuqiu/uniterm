export interface TableInfo {
  name: string
}

export interface ColumnInfo {
  name: string
  type: string
  nullable: boolean
  defaultVal: string
  isPrimary: boolean
}

export interface IndexInfo {
  name: string
  columns: string[]
  unique: boolean
}

export interface SchemaResult {
  columns: ColumnInfo[]
  indexes: IndexInfo[]
}

export interface QueryResultColumn {
  name: string
  type: string
}

export interface QueryResult {
  columns: QueryResultColumn[]
  rows: Record<string, any>[]
}

export interface ExecResult {
  affected: number
  lastInsertId: number
}

export interface HistoryEntry {
  id: string
  sql: string
  executedAt: string
  durationMs: number
  error?: string
  rowCount?: number
}
