import { useLayoutEffect, useRef } from 'react'
import type { NavFrame } from './nav'

export function navigationKey(stack: NavFrame[]): string {
  // Display names can change without changing the destination.
  return JSON.stringify(stack.map(({ label: _label, ...identity }: NavFrame & { label?: string }) => identity))
}

export function isNavigationReturn(previous: NavFrame[], next: NavFrame[]): boolean {
  return next.length < previous.length && navigationKey(previous.slice(0, next.length)) === navigationKey(next)
}

type Position = { x: number; y: number; contentX: number; contentY: number }
const top: Position = { x: 0, y: 0, contentX: 0, contentY: 0 }

export function useNavigationScroll(stack: NavFrame[], enabled: boolean) {
  const contentRef = useRef<HTMLDivElement>(null)
  const previous = useRef(stack)
  const position = useRef<Position>({ ...top })
  const positions = useRef(new Map<string, Position>())
  const key = navigationKey(stack)

  useLayoutEffect(() => {
    if (!enabled) return
    const old = previous.current
    const oldKey = navigationKey(old)
    if (oldKey !== key || (old !== stack && stack.length === 1)) {
      positions.current.set(oldKey, position.current)
      const returning = isNavigationReturn(old, stack)
      const target = returning ? positions.current.get(key) ?? top : top
      // A new root is a fresh visit, including reselecting the current tab.
      if (stack.length === 1 && !returning) positions.current.clear()
      window.scrollTo({ left: target.x, top: target.y, behavior: 'instant' })
      contentRef.current?.scrollTo({ left: target.contentX, top: target.contentY, behavior: 'instant' })
      position.current = { ...target }
      previous.current = stack
    }
    const content = contentRef.current
    const capture = () => {
      position.current = {
        x: window.scrollX, y: window.scrollY,
        contentX: content?.scrollLeft ?? 0, contentY: content?.scrollTop ?? 0,
      }
    }
    // Capture before React removes the tall outgoing view and clamps its scroll.
    const captureBeforeNavigation = () => capture()
    window.addEventListener('scroll', capture, { passive: true })
    content?.addEventListener('scroll', capture, { passive: true })
    document.addEventListener('click', captureBeforeNavigation, true)
    document.addEventListener('keydown', captureBeforeNavigation, true)
    return () => {
      window.removeEventListener('scroll', capture)
      content?.removeEventListener('scroll', capture)
      document.removeEventListener('click', captureBeforeNavigation, true)
      document.removeEventListener('keydown', captureBeforeNavigation, true)
    }
  }, [stack, key, enabled])

  return { contentRef, key }
}
