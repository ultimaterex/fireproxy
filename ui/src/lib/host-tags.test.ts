import { describe, expect, it } from 'vitest'
import { hostTagsAdd, hostTagsRemove } from './host-tags'

describe('hostTags', () => {
  it('adds idempotently', () => {
    expect(hostTagsAdd(['10'], '11').sort()).toEqual(['10', '11'])
    expect(hostTagsAdd(['10'], '10')).toEqual(['10'])
  })
  it('preserves existing policy memberships without mutating the input', () => {
    const tags = ['user-affiliated', 'restricted']
    expect(hostTagsAdd(tags, 'new')).toEqual(['user-affiliated', 'restricted', 'new'])
    expect(hostTagsAdd(tags, 'restricted')).toEqual(tags)
    expect(tags).toEqual(['user-affiliated', 'restricted'])
  })
  it('removes', () => {
    expect(hostTagsRemove(['10', '11'], '10')).toEqual(['11'])
  })
})
