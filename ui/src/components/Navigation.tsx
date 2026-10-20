import { useEffect, useState } from 'react'
import { Dialog } from 'radix-ui'
import {
  Archive,
  Bug,
  Menu,
  ChevronLeft,
  ChevronRight,
  Settings,
  X,
} from 'lucide-react'
import { NAV } from '@/lib/navigation-items'
import type { Tab } from '@/lib/types'
import { cn } from '@/lib/utils'

const NAV_COLLAPSED_KEY = 'fp-nav-collapsed'

function loadNavCollapsed(): boolean {
  try {
    return localStorage.getItem(NAV_COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}

type NavigationProps = {
  tab: Tab
  debugOn: boolean
  anonOn: boolean
  settingsHealthy: boolean
  selectTab: (tab: Tab) => void
  onSettings: () => void
}

function NavigationItems({
  tab,
  debugOn,
  anonOn,
  settingsHealthy,
  selectTab,
  onSettings,
  navCollapsed = false,
}: NavigationProps & { navCollapsed?: boolean }) {
  return (
    <nav
      aria-label="Main navigation"
      className="fp-navigation-items flex min-h-0 flex-1 flex-col overflow-y-auto"
    >
      {NAV.map((item) => {
        const Icon = item.icon
        const on = tab === item.id
        return (
          <button
            key={item.id}
            type="button"
            title={navCollapsed ? item.label : undefined}
            aria-label={item.label}
            aria-current={on ? 'page' : undefined}
            className={cn(
              'flex min-h-11 shrink-0 w-full items-center text-sm text-white/90 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-white hover:bg-white/5',
              navCollapsed ? 'justify-center px-0' : 'gap-3 px-6 text-left',
              on && 'bg-[#3f3f44] text-white',
            )}
            onClick={() => selectTab(item.id)}
          >
            <Icon className="size-6 shrink-0" />
            {!navCollapsed ? item.label : null}
          </button>
        )
      })}
      <div className="flex-1" />
      {anonOn ? (
        <div
          className={cn(
            'mx-3 mb-1 rounded-md bg-white/10 px-2 py-1 text-[11px] text-white/80',
            navCollapsed && 'mx-1 px-0 text-center',
          )}
          title="Anonymity"
        >
          {navCollapsed ? 'A' : 'Anonymity'}
        </div>
      ) : null}
      {debugOn ? (
        <button
          type="button"
          title={navCollapsed ? 'Debug' : undefined}
          aria-label="Debug"
          className={cn(
            'flex min-h-11 shrink-0 w-full items-center text-sm text-amber-200/90 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-white hover:bg-white/5',
            navCollapsed ? 'justify-center px-0' : 'gap-3 px-6 text-left',
            tab === 'debug' && 'bg-[#3f3f44] text-amber-100',
          )}
          onClick={() => selectTab('debug')}
        >
          <Bug className="size-6 shrink-0" />
          {!navCollapsed ? 'Debug' : null}
        </button>
      ) : null}
      <button
        type="button"
        title={navCollapsed ? 'Settings' : undefined}
        aria-label="Settings"
        aria-description={
          settingsHealthy
            ? 'Integrations healthy'
            : 'Integrations need attention'
        }
        aria-current={tab === 'settings' ? 'page' : undefined}
        className={cn(
          'relative flex min-h-11 shrink-0 w-full items-center text-sm text-white/90 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-white hover:bg-white/5',
          navCollapsed ? 'justify-center px-0' : 'gap-3 px-6 text-left',
          tab === 'settings' && 'bg-[#3f3f44] text-white',
        )}
        onClick={() => {
          onSettings()
          selectTab('settings')
        }}
      >
        <span className="relative shrink-0">
          <Settings className="size-6" />
          {navCollapsed ? (
            <span
              className={cn(
                'absolute -right-0.5 -top-0.5 size-2 rounded-full',
                settingsHealthy ? 'bg-emerald-500' : 'bg-red-500',
              )}
            />
          ) : null}
        </span>
        {!navCollapsed ? (
          <>
            Settings
            <span
              className={cn(
                'ml-auto size-2 rounded-full',
                settingsHealthy ? 'bg-emerald-500' : 'bg-red-500',
              )}
            />
          </>
        ) : null}
      </button>
      {debugOn ? (
        <button
          type="button"
          title={navCollapsed ? 'Legacy' : undefined}
          aria-label="Legacy"
          className={cn(
            'flex min-h-11 shrink-0 w-full items-center text-sm text-white/45 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-white hover:bg-white/5 hover:text-white/70',
            navCollapsed ? 'justify-center px-0' : 'gap-3 px-6 text-left',
            tab === 'legacy' && 'bg-[#3f3f44] text-white/80',
          )}
          onClick={() => selectTab('legacy')}
        >
          <Archive className="size-6 shrink-0" />
          {!navCollapsed ? 'Legacy' : null}
        </button>
      ) : null}
      <a
        href="https://github.com/ultimaterex/fireproxy"
        target="_blank"
        rel="noopener noreferrer"
        title="Source"
        aria-label="Source code"
        className={cn(
          'mb-2 flex min-h-11 shrink-0 items-center text-[11px] text-white/40 hover:text-white/70',
          navCollapsed ? 'justify-center px-0' : 'px-6',
        )}
      >
        {navCollapsed ? '<>' : 'Source'}
      </a>
    </nav>
  )
}

export function DesktopNavigation(props: NavigationProps) {
  const [collapsed, setCollapsed] = useState(loadNavCollapsed)
  return (
    <aside
      className={cn(
        'fp-desktop-navigation sticky top-0 z-30 hidden h-svh shrink-0 flex-col border-r border-white/10 bg-[#17171a] text-white md:flex',
        collapsed ? 'w-14' : 'w-50',
      )}
    >
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="flex min-h-14 shrink-0 items-center justify-center px-2">
          <div aria-label="FireProxy" title="FireProxy" className="truncate text-sm font-semibold tracking-tight">
            {collapsed ? 'FP' : 'FireProxy'}
          </div>
        </div>
        <NavigationItems {...props} navCollapsed={collapsed} />
      </div>
        <button
          type="button"
          className="absolute -right-3 top-1/2 flex h-12 w-6 -translate-y-1/2 items-center justify-center rounded-full border border-white/15 bg-[#202024] text-white/60 shadow-sm transition-colors hover:bg-[#303035] hover:text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white motion-reduce:transition-none"
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          aria-expanded={!collapsed}
          onClick={() => {
            setCollapsed((current) => {
              const next = !current
              try {
                localStorage.setItem(NAV_COLLAPSED_KEY, next ? '1' : '0')
              } catch {
                /* storage unavailable */
              }
              return next
            })
          }}
        >
          {collapsed ? (
            <ChevronRight className="size-4" />
          ) : (
            <ChevronLeft className="size-4" />
          )}
        </button>
    </aside>
  )
}

export function MobileNavigation(props: NavigationProps) {
  const [open, setOpen] = useState(false)
  useEffect(() => {
    const desktop = window.matchMedia('(min-width: 768px)')
    const closeOnDesktop = () => {
      if (desktop.matches) setOpen(false)
    }
    desktop.addEventListener('change', closeOnDesktop)
    return () => desktop.removeEventListener('change', closeOnDesktop)
  }, [])
  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger asChild>
        <button
          type="button"
          aria-label="Open navigation"
          className="mr-auto flex size-11 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-muted focus-visible:outline-2 focus-visible:outline-ring md:hidden"
        >
          <Menu className="size-5" />
        </button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="fp-navigation-backdrop fixed inset-0 z-40 bg-black/50" />
        <Dialog.Content
          aria-describedby={undefined}
          className="fp-mobile-navigation fixed inset-y-0 left-0 z-50 flex h-dvh w-72 max-w-[calc(100vw-3rem)] flex-col bg-[#17171a] text-white shadow-xl outline-none"
        >
          <div className="flex min-h-14 shrink-0 items-center justify-between gap-2 px-4">
            <Dialog.Title className="text-sm font-semibold tracking-tight">
              FireProxy
            </Dialog.Title>
            <Dialog.Close asChild>
              <button
                type="button"
                aria-label="Close navigation"
                className="flex size-11 items-center justify-center rounded-md text-white/70 hover:bg-white/5 hover:text-white focus-visible:outline-2 focus-visible:outline-white"
              >
                <X className="size-5" />
              </button>
            </Dialog.Close>
          </div>
          <NavigationItems
            {...props}
            selectTab={(tab) => {
              props.selectTab(tab)
              setOpen(false)
            }}
            onSettings={() => {
              props.onSettings()
              setOpen(false)
            }}
          />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
