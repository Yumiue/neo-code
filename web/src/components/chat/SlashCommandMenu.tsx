import { useEffect } from 'react'
import { type AnySlashCommand, isBuiltinCommand, isSkillCommand } from '@/utils/slashCommands'
import { Zap, BookOpen, Brain, Trash2, Layers, Sparkles, Terminal } from 'lucide-react'

interface SlashCommandMenuProps {
  commands: AnySlashCommand[]
  selectedIndex: number
  onSelect: (cmd: AnySlashCommand) => void
  onHover: (index: number) => void
  query: string
}

const iconMap: Record<string, React.ReactNode> = {
  compact: <Zap size={14} />,
  memo: <BookOpen size={14} />,
  remember: <Brain size={14} />,
  forget: <Trash2 size={14} />,
  skills: <Layers size={14} />,
}

function getCommandIcon(cmd: AnySlashCommand): React.ReactNode {
  if (isSkillCommand(cmd)) {
    return <Sparkles size={14} />
  }
  return iconMap[cmd.id] || <Terminal size={14} />
}

function highlightMatch(text: string, query: string): React.ReactNode {
  if (!query || query === '/') return text
  const lowerText = text.toLowerCase()
  const lowerQuery = query.toLowerCase().trim()
  const idx = lowerText.indexOf(lowerQuery)
  if (idx === -1) return text

  return (
    <>
      {text.slice(0, idx)}
      <span style={{ fontWeight: 700, color: 'var(--accent)' }}>{text.slice(idx, idx + lowerQuery.length)}</span>
      {text.slice(idx + lowerQuery.length)}
    </>
  )
}

/** Slash 命令浮动菜单 */
export default function SlashCommandMenu({ commands, selectedIndex, onSelect, onHover, query }: SlashCommandMenuProps) {
  useEffect(() => {
    const el = document.querySelector(`[data-slash-index="${selectedIndex}"]`)
    if (el) {
      el.scrollIntoView({ block: 'nearest' })
    }
  }, [selectedIndex])

  if (commands.length === 0) return null

  const builtinCmds = commands.filter(isBuiltinCommand)
  const skillCmds = commands.filter(isSkillCommand)

  return (
    <div style={styles.container}>
      {builtinCmds.length > 0 && (
        <div>
          <div style={styles.sectionLabel}>命令</div>
          {builtinCmds.map((cmd) => {
            const globalIndex = commands.indexOf(cmd)
            return (
              <CommandItem
                key={cmd.id}
                cmd={cmd}
                dataIndex={globalIndex}
                isSelected={selectedIndex === globalIndex}
                onSelect={() => onSelect(cmd)}
                onHover={() => onHover(globalIndex)}
                query={query}
              />
            )
          })}
        </div>
      )}

      {skillCmds.length > 0 && (
        <div>
          {builtinCmds.length > 0 && <div style={styles.divider} />}
          <div style={styles.sectionLabel}>技能</div>
          {skillCmds.map((cmd) => {
            const globalIndex = commands.indexOf(cmd)
            return (
              <CommandItem
                key={cmd.id}
                cmd={cmd}
                dataIndex={globalIndex}
                isSelected={selectedIndex === globalIndex}
                onSelect={() => onSelect(cmd)}
                onHover={() => onHover(globalIndex)}
                query={query}
              />
            )
          })}
        </div>
      )}
    </div>
  )
}

interface CommandItemProps {
  cmd: AnySlashCommand
  dataIndex: number
  isSelected: boolean
  onSelect: () => void
  onHover: () => void
  query: string
}

const CommandItem = ({ cmd, dataIndex, isSelected, onSelect, onHover, query }: CommandItemProps) => {
  const isSkill = isSkillCommand(cmd)

  return (
    <div
      data-slash-index={dataIndex}
      style={{
        ...styles.item,
        background: isSelected ? 'var(--accent-muted)' : 'transparent',
      }}
      onMouseEnter={onHover}
      onClick={onSelect}
    >
      <div style={{ ...styles.icon, color: isSelected ? 'var(--accent)' : 'var(--text-tertiary)' }}>
        {getCommandIcon(cmd)}
      </div>
      <div style={styles.info}>
        <div style={styles.usage}>
          {highlightMatch(cmd.usage, query)}
          {isSkill && cmd.active && (
            <span style={styles.activeBadge}>已激活</span>
          )}
        </div>
        <div style={styles.description}>{cmd.description}</div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    position: 'absolute',
    bottom: '100%',
    left: 0,
    marginBottom: 8,
    minWidth: 280,
    maxWidth: 360,
    maxHeight: 320,
    overflowY: 'auto',
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border-primary)',
    borderRadius: 'var(--radius-lg)',
    boxShadow: '0 4px 24px rgba(0,0,0,0.15)',
    zIndex: 100,
    padding: '6px 0',
  },
  sectionLabel: {
    fontSize: 11,
    fontWeight: 600,
    color: 'var(--text-tertiary)',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
    padding: '6px 12px 4px',
    userSelect: 'none',
  },
  divider: {
    height: 1,
    background: 'var(--border-primary)',
    margin: '4px 12px',
  },
  item: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    padding: '8px 12px',
    cursor: 'pointer',
    transition: 'background 0.1s',
    borderRadius: 'var(--radius-sm)',
    margin: '0 4px',
  },
  icon: {
    width: 28,
    height: 28,
    borderRadius: 'var(--radius-sm)',
    background: 'var(--bg-tertiary)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
    transition: 'color 0.1s',
  },
  info: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
    minWidth: 0,
    flex: 1,
  },
  usage: {
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--text-primary)',
    fontFamily: 'var(--font-mono)',
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  },
  description: {
    fontSize: 12,
    color: 'var(--text-tertiary)',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  activeBadge: {
    fontSize: 10,
    fontWeight: 500,
    color: 'var(--accent)',
    background: 'var(--accent-muted)',
    padding: '1px 5px',
    borderRadius: 'var(--radius-sm)',
    fontFamily: 'var(--font-ui)',
  },
}
