import { useState } from 'react'
import { ChevronDown } from 'lucide-react'

const mockModels = [
  { id: 'claude-4-sonnet', name: 'Claude 4 Sonnet', provider: 'Anthropic' },
  { id: 'claude-4-opus', name: 'Claude 4 Opus', provider: 'Anthropic' },
  { id: 'gpt-4o', name: 'GPT-4o', provider: 'OpenAI' },
  { id: 'gpt-4o-mini', name: 'GPT-4o Mini', provider: 'OpenAI' },
  { id: 'gemini-2.5-pro', name: 'Gemini 2.5 Pro', provider: 'Google' },
  { id: 'deepseek-v3', name: 'DeepSeek V3', provider: 'DeepSeek' },
]

export default function ModelSelector() {
  const [open, setOpen] = useState(false)
  const [selected, setSelected] = useState(mockModels[0])

  return (
    <div style={{ position: 'relative' }}>
      <button
        style={styles.selectorBtn}
        onClick={() => setOpen(!open)}
      >
        <span style={styles.modelName}>{selected.name}</span>
        <ChevronDown size={14} style={{ color: 'var(--text-tertiary)', transition: 'transform 0.15s', transform: open ? 'rotate(180deg)' : 'none' }} />
      </button>

      {open && (
        <div style={styles.dropdown} onMouseLeave={() => setOpen(false)}>
          {mockModels.map((m) => (
            <button
              key={m.id}
              style={{
                ...styles.dropdownItem,
                background: selected.id === m.id ? 'var(--bg-active)' : 'transparent',
              }}
              onClick={() => { setSelected(m); setOpen(false) }}
            >
              <span style={styles.dropdownName}>{m.name}</span>
              <span style={styles.dropdownProvider}>{m.provider}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  selectorBtn: {
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    padding: '4px 10px',
    borderRadius: 'var(--radius-md)',
    border: '1px solid var(--border-primary)',
    background: 'var(--bg-secondary)',
    color: 'var(--text-secondary)',
    fontSize: 12,
    fontFamily: 'var(--font-ui)',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
  modelName: {
    fontWeight: 500,
  },
  dropdown: {
    position: 'absolute',
    top: 'calc(100% + 6px)',
    right: 0,
    width: 220,
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border-primary)',
    borderRadius: 'var(--radius-md)',
    padding: 4,
    boxShadow: 'var(--shadow-3)',
    zIndex: 60,
  },
  dropdownItem: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    width: '100%',
    padding: '6px 10px',
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    color: 'var(--text-primary)',
    fontSize: 13,
    fontFamily: 'var(--font-ui)',
    cursor: 'pointer',
    textAlign: 'left',
    transition: 'all 0.15s',
  },
  dropdownName: {
    fontWeight: 500,
  },
  dropdownProvider: {
    fontSize: 11,
    color: 'var(--text-tertiary)',
  },
}
