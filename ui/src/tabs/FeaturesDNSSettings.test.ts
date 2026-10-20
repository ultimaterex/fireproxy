import { describe, expect, it } from 'vitest'

import { normalizeFeaturesView } from './FeaturesDNSSettings'

describe('normalizeFeaturesView', () => {
  it('defaults missing feature and DNS data', () => {
    expect(normalizeFeaturesView({ status: { state: 'lan-ok' } })).toEqual({
      status: { state: 'lan-ok' },
      features: [],
      dns: {
        unbound_summary: '',
        doh_enabled: false,
        doh_selected: [],
        config_writable: false,
      },
    })
  })

  it('defaults missing DNS fields independently', () => {
    expect(
      normalizeFeaturesView({
        status: { state: 'lan-ok' },
        dns: { doh_enabled: true },
      }).dns,
    ).toEqual({
      unbound_summary: '',
      doh_enabled: true,
      doh_selected: [],
      config_writable: false,
    })
  })
})
