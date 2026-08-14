# VRACK catalog

Topology → **VRACK**. Faceplates come from YAML. No file → **generic shelf stub** from live ports (`resolveGenericShelf`: RJ45 + SFP; &gt;24 RJ45 → dual rail). APs use the Wireless band, not YAML.

```
catalog/kit.yaml                         shared sizes
catalog/<manufacturer>/<type>/<product>.yaml
```

`manufacturer`: `unifi` | `firewalla` | `tplink`  
`type`: `switch` | `ap` | `box`

Path fills those fields if omitted. Example: `catalog/unifi/switch/usw-pro-hd-24.yaml`.

## kit.yaml

One file. All products share it.

| key | meaning |
|---|---|
| `scale_px_per_in` | 48 — 19″ face = 912 px, 1U = 84 px |
| `u.width_in` / `u.height_in` | 19 × 1.75 |
| `screen.diagonal_in` | front display (1.3″). Drawn as the maker logo |
| `rj45.width_in` | receptacle face (0.63″) |
| `sfp.width_mm` / `sfp.height_mm` | cage width 13.4; height_mm unused (stack uses screen band) |
| `keystone.width_in` | blank keystone face (0.58) |

Fixed by the engine (do not set per product):

- `screen` is vertically centered on the 1U
- `rj45` height = half the screen; bottom flush with screen; top at screen mid
- stacked `sfp` height = screen
- `trailer` width = one SFP

## Product file

```yaml
manufacturer: unifi
type: switch
product: usw-pro-hd-24
match:
  model:
    - USW Pro HD 24
    - USW-Pro-HD-24
ru: 1                 # rack units; default 1
form: rack            # rack | shelf; default rack
width_in: 12.99       # chassis only; omit = full 19″. Remainder is flush ears
height_in: 0.83       # shelf front height; rack ignores this (uses ru)
align: left           # left | center | right — short body placement
ear_in: 0.6           # short ear on the non-extension side (with align left/right)
blocks: []
```

`match.model` is compared to live `Switch.model` / `name` (case and punctuation ignored). Alias length ≥ 8 can substring-match (`USW-Pro-HD-24` → `USW-Pro-HD-24-PoE`).

`form: shelf` draws a desktop faceplate on the shelf tray (Flex). Uses `width_in` × `height_in` at the same px/in as the rack — no ears.

`width_in` shorter than 19″ + `align: center` (default) → flush ears both sides (Switch X).

`align: left` → body flush left, blank extension fills the right (Pro Max 16 is 12.8″ / 325.1 mm). Optional `ear_in` adds a short ear on the non-extension side.

Shelf examples: Flex 2.5G 5 is 4.61″ × 1.1″; Flex 2.5G 8 is 8.38″ × 1.1″ (28 mm front height).

`window_in` is a centered device cutout on a full-width 1U shelf (Gold SE is 5.51″ / 140 mm). Keystone banks sit in the wings.

## blocks

Left → right on the 1U. Kinds:

### `screen`

Maker logo on a display square, sized from `kit.screen`. Present → copper *fills* the bay (UniFi).

### `badge`

Compact maker mark, no display square. Use on bodies that print a logo (Switch X, Gold SE).

### `keystone`

```yaml
- kind: keystone
  side: left         # left | right
  rail: center
  slots: 8
```

Blank keystone faces. Only laid out when `window_in` is set. Spread across that wing.

### `rj45`

```yaml
- kind: rj45
  rail: low          # high | center | low (omit with order: columns)
  order: columns     # columns = dual-rail odd/even pairs; omit/rows = single rail L→R
  bank: 8            # split into groups of N; omit = one group
  ports: 1-24        # range, list, or "1-8,17,18"
  numbers: below     # above | below (columns overrides: odd below, even above)
  gap: 12            # extra px between jacks when packed (Switch X)
  lead: 16           # extra px before this bank (shelf: 1–8 → PoE IN)
```

`low` = bottom of the screen band. `center` = vertical mid of the 1U. `high` = top of the screen band. No `screen` → ports pack at kit width and sit just left of the SFP (right-aligned in the chassis).

`order: columns` stacks pairs in columns (like SFP columns). Default: odd ports on `low` with numbers below, even on `high` with numbers above. Set `odds: high` for top-row odds (TL-SG1016PE: 1/3/5… above, 2/4/6… below). `bank: 8` still inserts a gap between groups (TL-SG1016PE: 1–8 PoE then 9–16).

### `sfp`

```yaml
- kind: sfp
  cols: 2
  order: columns     # columns = 25/26 then 27/28; rows = left-to-right then down
  ports: [25, 26, 27, 28]
  rail: low          # single-row only; 2-row still uses high/low from row
  gap: 22            # px before the SFP bank (parks it on the side)
  numbers:
    high: above
    low: below
```

Row 0 = `high`, row 1 = `low`. One row (`cols` = port count) sits on `center` with numbers above (Switch X 9–12). `cols: 4` + `order: columns` + ports `1-8` is Aggregation (odds high, evens low, parked on the right). No `rj45` block → empty bay in the middle.

### `trailer`

Empty bay after the last ports, one SFP wide. Nothing is drawn. Omit it to let copper/SFP run to the right pad.

## Add a switch

1. Drop `catalog/<maker>/switch/<slug>.yaml`
2. Set `match.model` to the UniFi / Firewalla model string
3. Describe `blocks` (copy `unifi/switch/usw-pro-hd-24.yaml` and edit)
4. Rebuild UI

Boxes (`type: box`) match `Box.license` / `model` (Gold SE shelf is `firewalla/box/gold-se.yaml`).

Live up/down still comes from topology. YAML does not mark uplinks.
