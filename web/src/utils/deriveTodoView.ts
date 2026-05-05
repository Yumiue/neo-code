import type { TodoSnapshot, TodoViewItem } from '@/api/protocol'
import type { TodoHistoryEntry } from '@/stores/useRuntimeInsightStore'

export type TodoViewRow = TodoViewItem & { isStale: boolean }

const DEFAULT_CAP = 5

/**
 * 把当前 snapshot 与累积 history 合成为 TodoStrip 要渲染的有序行列表。
 *
 * 规则:
 * - active 段保持 snapshot.items 原序;
 * - stale 段按 lastSeenAt 倒序(最近被替换的优先);
 * - 行数上限 = max(cap, activeCount);active 占满上限时不展示 stale。
 */
export function deriveTodoView(
  snapshot: TodoSnapshot | null,
  history: Record<string, TodoHistoryEntry>,
  cap: number = DEFAULT_CAP,
): TodoViewRow[] {
  const activeItems = snapshot?.items ?? []
  const activeIds = new Set(activeItems.map((i) => i.id))

  const staleEntries = Object.values(history)
    .filter((h) => !activeIds.has(h.id))
    .sort((a, b) => b.lastSeenAt - a.lastSeenAt)

  const limit = Math.max(cap, activeItems.length)
  const slotsLeft = Math.max(0, limit - activeItems.length)

  const activeRows: TodoViewRow[] = activeItems.map((item) => ({ ...item, isStale: false }))
  const staleRows: TodoViewRow[] = staleEntries.slice(0, slotsLeft).map((entry) => {
    // 剥掉 lastSeenAt/firstSeenAt 这俩 history-only 字段,只把 TodoViewItem 字段透传出去
    const { lastSeenAt: _ls, firstSeenAt: _fs, ...item } = entry
    void _ls
    void _fs
    return { ...item, isStale: true }
  })

  return [...activeRows, ...staleRows]
}
