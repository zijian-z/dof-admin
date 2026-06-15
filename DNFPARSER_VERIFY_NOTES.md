# DNF Parser PVF Verification Notes

## Verification Input

- Input PVF: `extracted/Script.pvf`
- Input size: `205,695,984` bytes
- Verification report: `dnfparser_verify_output/dnfparser_verify_report.json`
- Representative exported outputs: `dnfparser_verify_output/*.json` and `dnfparser_verify_output/*.source.txt`

The verification test opens `Script.pvf`, loads the file tree, parses `stringtable.bin`, loads `n_string.lst`, then walks every PVF entry through `LoadScript`.

## Script.pvf Resource Boundary

`Script.pvf` is primarily a script/config package. It contains game data such as indexes, item/skill/monster/map definitions, action scripts, animation descriptions, string tables, and references to image resources.

It does not normally contain the real texture pixel data.

The actual image resources are stored in NPK packages, where `.img` entries are decoded by the NPK side of `dnfparser.go` via `OpenNpk`, `LoadImg`, and texture decoding.

In other words:

- `Script.pvf`: scripts and resource references.
- `.ani`: animation metadata that references `.img` resources.
- `.npk`: packed image/resource data.
- `.img`: image/texture resource inside NPK, not usually embedded directly in `Script.pvf`.

`Script.pvf` may contain paths such as:

```text
Character/Swordman/Equipment/Avatar/skin/sm_body%04d.img
```

That path is an image resource reference used by an animation frame. The actual image bytes should be resolved from NPK resource packs.

## ANI Role

In this parser, `.ani` files are DNF animation scripts. They define how animation frames are played and how those frames relate to image resources and gameplay metadata.

Common parsed fields include:

- `[FRAME MAX]`: total frame count.
- `[LOOP]`: loop flag.
- `[FRAME]`: list of per-frame entries.
- `[IMAGE]`: referenced image path/index for the frame.
- `[IMAGE POS]`: frame draw offset.
- `[DELAY]`: frame duration.
- `[RGBA]`: color/alpha values.
- `[IMAGE RATE]`: scale values.
- `[IMAGE ROTATE]`: rotation value.
- `[GRAPHIC EFFECT]`: graphic effect parameters.
- `[DAMAGE BOX]`: hit-receive collision box.
- `[ATTACK BOX]`: attack collision box.
- `[PLAY SOUND]`: sound triggered by the frame.
- `[SPECTRUM]`: spectrum/afterimage-style effect data.
- `[CLIP]`: clipping rectangle.
- `[FLIP TYPE]`: flip mode.

So `.ani` is not just a list of pictures. Some ANI files also carry gameplay-relevant frame data, especially attack and damage boxes.

Example exported sample:

```json
{"[FRAME MAX]":4,"[LOOP]":1,"[FRAME]":[{"[IMAGE]":["",-1],"[IMAGE POS]":[-279,-382],"[DELAY]":1000},{"[IMAGE]":["",-1],"[IMAGE POS]":[-279,-382],"[DELAY]":1000},{"[IMAGE]":["",-1],"[IMAGE POS]":[-279,-382],"[DELAY]":1000},{"[IMAGE]":["",-1],"[IMAGE POS]":[-279,-382],"[DELAY]":1000}]}
```

## Verification Summary

Latest full verification result:

- PVF tree count in header: `370,666`
- Parsed tree files: `370,666`
- `stringtable.bin` entries: `461,796`
- n_string files loaded: `26`
- Total PVF entries walked through script loading: `370,666`
- Script load panics: `0`
- Empty structured outputs: `501`
- ANI partial parse outputs with `[PARSE ERROR]`: `967`
- Full test command: `go test ./...`
- Final test result: passed

The main parser flow is working for the extracted `Script.pvf`: opening the PVF, decoding the tree, resolving string tables, loading n_string, and walking all scripts no longer crashes.

## ANI Partial Parse Cases

There are `209,635` `.ani` files in this `Script.pvf`.

Of those, `967` are now returned as partial structured outputs containing a `[PARSE ERROR]` field. That is about `0.46%` of ANI files.

These are no longer fatal to the main flow. The parser returns whatever was parsed before the malformed/special section and marks the result with `[PARSE ERROR]`.

The partial parse list is written to:

```text
dnfparser_verify_output/dnfparser_parse_errors.tsv
```

Observed error patterns are mostly:

- Empty ANI data.
- Declared frame/parameter structure reaches end-of-file before all expected fields are present.
- Special or older ANI variants not fully modeled by the current parser.
- Large invalid-looking string/length reads after parser desynchronization.

Top-level path distribution for the `967` partial ANI cases:

```text
295 monster
255 passiveobject
216 ui
138 equipment
36  character
10  npc
5   common
4   dungeon
4   creature
1   aicharacter
1   stagemap
1   stackable
1   map
```

High-frequency subdirectories:

```text
138 passiveobject/actionobject/monster
127 equipment/character/thief
45  monster/newmonsters/anton_normal
37  ui/event/ontimeevent
32  passiveobject/actionobject/spc
31  ui/bluemarble/animation
30  passiveobject/character/swordman
27  monster/newmonsters/anton
27  monster/anton/phase3
23  ui/tournament/animation
21  monster/event/bluemarble
20  monster/anton/phase1
18  ui/event/twdf
18  monster/newmonsters/timegate
12  monster/lotus/lotusani
```

This means the remaining partial ANI cases are concentrated in monster animations, passive objects, UI/event animations, and equipment/avatar animation resources. They are not blocking the core PVF script flow.

## Parser Changes Made During Verification

The verification exposed two practical issues:

1. JSON string escaping was incomplete.

   The custom JSON writer previously escaped only a small set of characters. Some `.str` output contained control characters that made JSON invalid. `writeJSONString` now delegates string escaping to `encoding/json`.

2. ANI `[SPECTRUM]` data was not consumed.

   The original Java parser marked `[SPECTRUM]` as TODO and returned `null` without consuming payload bytes. In this Go port that caused frame parsing to become misaligned for some real ANI files. The Go parser now consumes the observed 15-byte spectrum payload and outputs it as a byte array.

Additionally, ANI parsing now guards the main flow from empty/truncated/special ANI files by returning a structured result with `[PARSE ERROR]` instead of panicking.

## Current Interpretation

For the current goal, the parser input/output is acceptable for the main PVF workflow:

- PVF loading is correct enough to read the full tree.
- `stringtable.bin` and `n_string.lst` are loaded.
- All entries can be walked without panic.
- Representative outputs for `bin`, `lst`, `str`, `ani`, `ui`, and normal script files are written to disk.
- Remaining ANI issues are isolated and reported as partial parse records.

If exact rendering or combat-frame fidelity is needed later, the next step should be focused ANI reverse engineering using `dnfparser_parse_errors.tsv` as the input list, plus cross-checking against the related NPK `.img` resources.
