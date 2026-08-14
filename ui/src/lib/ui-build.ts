export const UI_BUILD_POLL_MS = 2 * 60 * 1000
export const UI_UPDATE_FORCE_EVENT = 'fp-force-ui-update'

export const bootUiBuild: string =
  typeof __UI_BUILD__ !== 'undefined' && __UI_BUILD__ ? __UI_BUILD__ : ''

/** Debug helper: show the reload toast regardless of version.json. */
export function forceUiUpdateBanner() {
  window.dispatchEvent(new Event(UI_UPDATE_FORCE_EVENT))
}

/** Returns true when the served build differs from the boot build. */
export async function checkUiUpdate(): Promise<boolean> {
  if (import.meta.env.DEV || !bootUiBuild) return false
  try {
    const r = await fetch(`/version.json?t=${Date.now()}`, { cache: 'no-store' })
    if (!r.ok) return false
    const body = (await r.json()) as { build?: string }
    return typeof body.build === 'string' && body.build !== '' && body.build !== bootUiBuild
  } catch {
    return false
  }
}
