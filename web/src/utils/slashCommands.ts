/**
 * Slash Command 定义、解析与匹配工具模块
 * 与 TUI 端 internal/tui/core/app/commands.go 对齐
 */

export interface SlashCommand {
  id: string
  usage: string
  description: string
  hasArgument: boolean
  argumentPlaceholder?: string
}

export interface SkillSlashCommand extends SlashCommand {
  isSkill: true
  skillId: string
  active: boolean
}

export type AnySlashCommand = SlashCommand | SkillSlashCommand

export const builtinSlashCommands: SlashCommand[] = [
  {
    id: 'help',
    usage: '/help',
    description: '显示所有可用命令',
    hasArgument: false,
  },
  {
    id: 'compact',
    usage: '/compact',
    description: '压缩当前会话上下文',
    hasArgument: false,
  },
  {
    id: 'memo',
    usage: '/memo',
    description: '显示持久化备忘录索引',
    hasArgument: false,
  },
  {
    id: 'remember',
    usage: '/remember',
    description: '保存持久化备忘录',
    hasArgument: true,
    argumentPlaceholder: '内容',
  },
  {
    id: 'forget',
    usage: '/forget',
    description: '按关键词删除备忘录',
    hasArgument: true,
    argumentPlaceholder: '关键词',
  },
  {
    id: 'skills',
    usage: '/skills',
    description: '列出可用技能并管理',
    hasArgument: false,
  },
]

/** 所有内置命令的 usage 集合，用于快速判断 */
const builtinUsages = new Set(builtinSlashCommands.map((c) => c.usage))

/**
 * 解析 slash command 输入
 * 输入 "/remember 用户名是 Alice" → { command: '/remember', argument: '用户名是 Alice' }
 */
export function parseSlashCommand(input: string): { command: string; argument: string } | null {
  const trimmed = input.trim()
  if (!trimmed.startsWith('/')) return null

  const firstSpace = trimmed.indexOf(' ')
  if (firstSpace === -1) {
    return { command: trimmed.toLowerCase(), argument: '' }
  }

  const command = trimmed.slice(0, firstSpace).toLowerCase()
  const argument = trimmed.slice(firstSpace + 1).trim()
  return { command, argument }
}

/**
 * 判断输入是否是 slash command（以 / 开头且不止一个字符）
 */
export function isSlashCommand(input: string): boolean {
  const trimmed = input.trim()
  return trimmed.startsWith('/') && trimmed.length > 1
}

/**
 * 根据输入过滤匹配命令列表
 * 输入 "/com" → 过滤出 /compact
 */
export function matchSlashCommands(input: string, commands: AnySlashCommand[]): AnySlashCommand[] {
  if (!isSlashCommand(input)) return []

  const query = input.trim().toLowerCase()
  if (query === '/') return commands

  return commands.filter(
    (cmd) =>
      cmd.usage.toLowerCase().includes(query) ||
      cmd.description.toLowerCase().includes(query) ||
      cmd.id.toLowerCase().includes(query.slice(1)),
  )
}

/**
 * 判断输入是否匹配一个已知的完整内置命令
 * 用于决定 Enter 是执行命令还是发送普通消息
 */
export function isKnownSlashCommand(input: string): boolean {
  const parsed = parseSlashCommand(input)
  if (!parsed) return false
  return builtinUsages.has(parsed.command)
}

/**
 * 判断是否为内置命令（非技能命令）
 */
export function isBuiltinCommand(cmd: AnySlashCommand): cmd is SlashCommand {
  return !('isSkill' in cmd)
}

/**
 * 判断是否为技能命令
 */
export function isSkillCommand(cmd: AnySlashCommand): cmd is SkillSlashCommand {
  return 'isSkill' in cmd && cmd.isSkill
}
