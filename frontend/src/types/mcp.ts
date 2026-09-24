// MCP (external AI agents) frontend types.

export interface MCPApprovalRequest {
  id: string
  client: string
  connection?: string
  command?: string
  createdAt?: number
}

export interface MCPStatus {
  running: boolean
  port: number
}

export interface MCPTools {
  exec: boolean
  terminal: boolean
  files: boolean
}

export interface MCPSettings {
  enabled: boolean
  port?: number
  policy?: string
  tools: MCPTools
}

export const DEFAULT_MCP_SETTINGS: MCPSettings = {
  enabled: false,
  policy: 'confirm_all',
  tools: { exec: true, terminal: false, files: false },
}

/** Client onboarding snippets shown in the settings page. */
export function mcpClientConfigs(port: number, token: string): { name: string; cmd: string }[] {
  const url = `http://127.0.0.1:${port}/mcp`
  return [
    { name: 'Claude Code', cmd: `claude mcp add --transport http uniterm ${url} --header "Authorization: Bearer ${token}"` },
    { name: 'Codex CLI', cmd: `[mcp_servers.uniterm]\nurl = "${url}"\nhttp_headers = { "Authorization" = "Bearer ${token}" }` },
    { name: 'Kimi CLI', cmd: `kimi mcp add --transport http uniterm ${url} --header "Authorization: Bearer ${token}"` },
  ]
}
