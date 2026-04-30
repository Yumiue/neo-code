import { useState, useEffect, useCallback } from 'react'
import { useUIStore } from '@/stores/useUIStore'
import { useGatewayAPI } from '@/context/RuntimeProvider'
import { type FileEntry } from '@/api/protocol'
import {
  Folder,
  FolderOpen,
  File,
  FileCode,
  FileText,
  FileJson,
  ChevronRight,
  PanelRightClose,
  Loader2,
} from 'lucide-react'

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

interface FileTreeNode {
  entry: FileEntry
  children?: FileTreeNode[]
}

function buildFileTree(entries: FileEntry[]): FileTreeNode[] {
  const rootNodes: FileTreeNode[] = []
  const dirMap = new Map<string, FileTreeNode>()

  // 先创建所有目录节点
  for (const entry of entries) {
    if (entry.is_dir) {
      const node: FileTreeNode = { entry, children: [] }
      dirMap.set(entry.path, node)
    }
  }

  // 再分配所有节点到父目录
  for (const entry of entries) {
    const parentPath = entry.path.split('/').slice(0, -1).join('/')
    if (parentPath && dirMap.has(parentPath)) {
      const parent = dirMap.get(parentPath)!
      if (entry.is_dir) {
        parent.children!.push(dirMap.get(entry.path)!)
      } else {
        parent.children!.push({ entry })
      }
    } else if (!parentPath) {
      if (entry.is_dir) {
        rootNodes.push(dirMap.get(entry.path)!)
      } else {
        rootNodes.push({ entry })
      }
    }
  }

  return rootNodes
}

function FileTreeItem({ node, depth = 0 }: { node: FileTreeNode; depth?: number }) {
  const [expanded, setExpanded] = useState(true)
  const isFolder = node.entry.is_dir

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
          {isFolder ? (expanded ? <FolderOpen size={14} /> : <Folder size={14} />) : getFileIcon(node.entry.name)}
        </span>
        <span style={styles.treeName}>{node.entry.name}</span>
      </button>
      {isFolder && expanded && node.children?.map((child, i) => (
        <FileTreeItem key={i} node={child} depth={depth + 1} />
      ))}
    </div>
  )
}

export default function FileTreePanel() {
  const toggleFileTreePanel = useUIStore((s) => s.toggleFileTreePanel)
  const gatewayAPI = useGatewayAPI()
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [currentPath, setCurrentPath] = useState('')

  const loadFiles = useCallback(async (path: string = '') => {
    if (!gatewayAPI) return
    setLoading(true)
    setError('')
    try {
      const result = await gatewayAPI.listFiles({ path })
      setEntries(result.payload.files)
      setCurrentPath(path)
    } catch (err) {
      const msg = err instanceof Error ? err.message : '加载文件列表失败'
      setError(msg)
      console.error('listFiles failed:', err)
    } finally {
      setLoading(false)
    }
  }, [gatewayAPI])

  useEffect(() => {
    loadFiles()
  }, [loadFiles])

  const treeNodes = buildFileTree(entries)

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <div style={styles.headerTop}>
          <span style={styles.headerTitle}>工作区</span>
          <button style={styles.closeBtn} onClick={toggleFileTreePanel} title="关闭文件目录">
            <PanelRightClose size={16} />
          </button>
        </div>
        <div style={styles.headerPath}>{currentPath || '.'}</div>
      </div>

      <div style={styles.scrollArea}>
        {loading && (
          <div style={styles.emptyState}>
            <Loader2 size={16} style={{ animation: 'spin 1s linear infinite' }} />
            <span style={styles.emptyText}>加载中...</span>
          </div>
        )}
        {!loading && error && (
          <div style={styles.emptyState}>
            <span style={{ ...styles.emptyText, color: 'var(--error)' }}>加载失败: {error}</span>
          </div>
        )}
        {!loading && !error && treeNodes.length === 0 && (
          <div style={styles.emptyState}>
            <span style={styles.emptyText}>工作区为空</span>
          </div>
        )}
        {!loading && !error && treeNodes.map((node, i) => (
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
  emptyState: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    padding: '20px 8px',
    color: 'var(--text-tertiary)',
  },
  emptyText: {
    fontSize: 12,
    fontFamily: 'var(--font-ui)',
  },
}
