import { createContext, useCallback, useContext, useRef, useState, type Dispatch, type SetStateAction } from 'react'

export type NavigationViewState = Map<string, unknown>
export const NavigationViewStateContext = createContext<NavigationViewState | null>(null)

// Keep view preferences while a detail replaces its originating list. Transient
// menus and mutation state deliberately continue to use ordinary component state.
export function useNavigationViewState<T>(key: string, initial: T | (() => T)): [T, Dispatch<SetStateAction<T>>] {
  const cache = useContext(NavigationViewStateContext)
  const [value, setValue] = useState<T>(() => cache?.has(key)
    ? cache.get(key) as T
    : typeof initial === 'function' ? (initial as () => T)() : initial)
  const current = useRef(value)
  const update: Dispatch<SetStateAction<T>> = useCallback((action) => {
    // Resolve outside React's updater so caching is not a render-time side effect.
    const previous = current.current
    const next = typeof action === 'function' ? (action as (value: T) => T)(previous) : action
    cache?.set(key, next)
    current.current = next
    setValue(next)
  }, [cache, key])
  return [value, update]
}
