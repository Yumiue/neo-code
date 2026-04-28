import { useState } from 'react'
import { useUIStore } from '@/store/useUIStore'
import {
  Folder,
  FolderOpen,
  File,
  FileCode,
  FileText,
  FileJson,
  ChevronRight,
  PanelRightClose,
} from 'lucide-react'

interface FileNode {
  name: string
  type: 'folder' | 'file'
  children?: FileNode[]
}

const fileIconMap: Record<string, React.ReactNode> = {
  js: <FileCode size={14} />,
  jsx: <FileCode size={14} />,
  ts: <FileCode size={14} />,
  tsx: <FileCode size={14} />,
  json: <FileJson size={14} />,
  md: <FileText size={14} />,
}

function getFileIcon(filename: string) {
  const ext = filename.split('.').pop() || ''
  return fileIconMap[ext] || <File size={14} />
}

function FileTreeItem({ node, depth = 0 }: { node: FileNode; depth?: number }) {
  const [expanded, setExpanded] = useState(true)
  const isFolder = node.type === 'folder'

  return (
    <div>
      <button
        style={{
          ...styles.treeItem,
          paddingLeft: 8 + depth * 14,
        }}
        onClick={() => isFolder && setExpanded(!expanded)}
      >
        {isFolder && (
          <span style={{ ...styles.chevron, transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)' }}>
            <ChevronRight size={12} />
          </span>
        )}
        <span style={styles.treeIcon}>
          {isFolder ? (expanded ? <FolderOpen size={14} /> : <Folder size={14} />) : getFileIcon(node.name)}
        </span>
        <span style={styles.treeName}>{node.name}</span>
      </button>
      {isFolder && expanded && node.children?.map((child, i) => (
        <FileTreeItem key={i} node={child} depth={depth + 1} />
      ))}
    </div>
  )
}

export default function FileTreePanel() {
  const toggleFileTreePanel = useUIStore((s) => s.toggleFileTreePanel)

  // mock file tree data
  const mockFileTree: FileNode[] = []

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <div style={styles.headerTop}>
          <span style={styles.headerTitle}>工作区</span>
          <button style={styles.closeBtn} onClick={toggleFileTreePanel} title="关闭文件目录">
            <PanelRightClose size={16} />
          </button>
        </div>
        <div style={styles.headerPath}>~/projects/neocode-auth</div>
      </div>

      <div style={styles.scrollArea}>
        {mockFileTree.map((node, i) => (
          <FileTreeItem key={i} node={node} />
        ))}
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    height: '100%',
    background: 'var(--bg-secondary)',
  },
  header: {
    padding: '12px 14px',
    borderBottom: '1px solid var(--border-primary)',
    flexShrink: 0,
  },
  headerTop: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 4,
  },
  headerTitle: {
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--text-primary)',
  },
  headerPath: {
    fontSize: 11,
    color: 'var(--text-tertiary)',
    fontFamily: 'var(--font-mono)',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  closeBtn: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 24,
    height: 24,
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-tertiary)',
    cursor: 'pointer',
  },
  scrollArea: {
    flex: 1,
    overflowY: 'auto',
    padding: '6px 4px',
  },
  treeItem: {
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    width: '100%',
    padding: '4px 8px',
    borderRadius: 'var(--radius-sm)',
    border: 'none',
    background: 'transparent',
    color: 'var(--text-secondary)',
    fontSize: 12,
    cursor: 'pointer',
    fontFamily: 'var(--font-ui)',
    textAlign: 'left',
    transition: 'all 0.15s',
  },
  chevron: {
    display: 'flex',
    transition: 'transform 0.2s',
    color: 'var(--text-tertiary)',
    width: 14,
    flexShrink: 0,
  },
  treeIcon: {
    display: 'flex',
    flexShrink: 0,
    color: 'var(--text-tertiary)',
  },
  treeName: {
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    fontFamily: 'var(--font-mono)',
    fontSize: 11,
  },
}
