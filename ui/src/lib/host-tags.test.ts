import { describe, expect, it } from 'vitest'
import { hostTagsAdd } from './host-tags'

describe('hostTagsAdd', () => {
  it('preserves policy memberships and avoids duplicate assignment', () => {
    const tags = ['user-affiliated', 'restricted']
    expect(hostTagsAdd(tags, 'new')).toEqual(['user-affiliated', 'restricted', 'new'])
    expect(hostTagsAdd(tags, 'restricted')).toEqual(tags)
    expect(tags).toEqual(['user-affiliated', 'restricted'])
  })
})
