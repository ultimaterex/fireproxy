import { describe, expect, it } from 'vitest'
import { hostTagsAdd, hostTagsRemove } from './host-tags'

describe('hostTags', () => {
  it('adds idempotently', () => {
    expect(hostTagsAdd(['10'], '11').sort()).toEqual(['10', '11'])
    expect(hostTagsAdd(['10'], '10')).toEqual(['10'])
  })
  it('removes', () => {
    expect(hostTagsRemove(['10', '11'], '10')).toEqual(['11'])
  })
})
