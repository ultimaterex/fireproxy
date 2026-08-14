import { Tag as TagIcon, User } from 'lucide-react'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { Tag, ViewMode } from '@/lib/types'

type GroupRow = Tag & { count: number; kind?: 'user' | 'group' }

export function GroupsTab({
  mode,
  groups,
  onSelectGroup,
}: {
  mode: ViewMode
  groups: GroupRow[]
  onSelectGroup: (id: string) => void
}) {
  const ranked = [...groups].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))

  if (mode === 'list') {
    return (
      <Card className="gap-0 py-0">
        <CardHeader className="border-b py-4">
          <CardTitle className="text-sm">Groups</CardTitle>
          <CardDescription>{groups.length}</CardDescription>
        </CardHeader>
        <CardContent className="px-0">
          {groups.length === 0 ? (
            <p className="px-6 py-8 text-sm text-muted-foreground">No groups</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>ID</TableHead>
                  <TableHead className="text-right">Devices</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((g) => (
                  <TableRow key={g.id}>
                    <TableCell>
                      <button
                        type="button"
                        className="inline-flex items-center gap-1.5 text-left hover:underline"
                        onClick={() => onSelectGroup(g.id)}
                      >
                        {g.kind === 'user' ? (
                          <User className="size-3.5 text-muted-foreground" />
                        ) : (
                          <TagIcon className="size-3.5 text-muted-foreground" />
                        )}
                        {g.name}
                      </button>
                    </TableCell>
                    <TableCell className="font-mono text-muted-foreground">{g.id}</TableCell>
                    <TableCell className="text-right font-mono tabular-nums">
                      {g.count}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    )
  }

  if (groups.length === 0) {
    return <p className="text-sm text-muted-foreground">No groups</p>
  }

  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {ranked.map((g) => (
        <button key={g.id} type="button" className="text-left" onClick={() => onSelectGroup(g.id)}>
          <Card className="h-full gap-2 py-4 transition-colors hover:bg-accent/40">
            <CardHeader className="px-5">
              <div className="flex items-baseline justify-between gap-2">
                <CardTitle className="inline-flex min-w-0 items-center gap-1.5 text-base">
                  {g.kind === 'user' ? (
                    <User className="size-4 shrink-0 text-muted-foreground" />
                  ) : (
                    <TagIcon className="size-4 shrink-0 text-muted-foreground" />
                  )}
                  <span className="truncate">{g.name}</span>
                </CardTitle>
                <span className="font-mono text-lg tabular-nums">{g.count}</span>
              </div>
              <CardDescription className="font-mono">{g.id}</CardDescription>
            </CardHeader>
          </Card>
        </button>
      ))}
    </div>
  )
}
