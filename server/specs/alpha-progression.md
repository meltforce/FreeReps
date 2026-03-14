# Alpha Progression — CSV Format Spec

## Overview

Alpha Progression exports workout data as semicolon-delimited CSV with European locale.
This supplements Apple Health workout data with detailed set/rep/weight information.

## Format

- Delimiter: `;` (semicolon)
- Decimal separator: `,` (comma) — European locale
- Encoding: UTF-8
- Line ending: `\n`
- Quoting: double quotes around fields containing special chars

## Structure

The file contains one or more sessions separated by blank lines.

### Session Header

```
"<Session Name>";"<Date> <Time>";"<Duration>"
```

Example:
```
"Legs · Day 2 · Week 4 · Push-Pull-Legs";"2026-02-19 4:54 h";"1:02 hr"
```

Fields:
- **Session name**: Quoted string, often `"<Body Part> · Day N · Week N · <Program>"`
- **Date + time**: `"YYYY-MM-DD H:MM h"` — 24h format, space before `h`
- **Duration**: `"H:MM hr"` or `"MM:SS hr"`

### Exercise Header

```
"<N>. <Exercise Name> · <Equipment> · <Target Reps> reps"[;"<Warmup Info>"]
```

Examples:
```
"1. Hack Squats · Machine · 8 reps";"WU1 · 37,5 kg · 9 reps<br>WU2 · 72,5 kg · 7 reps"
"4. Reverse Lunges · Dumbbells · 10 reps"
```

Fields:
- **Exercise number**: Sequential integer
- **Exercise name**: Free text
- **Equipment**: Free text (e.g. `Machine`, `Barbell`, `Dumbbells`, `Smith machine`, `Cable`, `Bodyweight`)
- **Target reps**: Integer
- **Optional modifiers**: `· 2 dropsets` appended
- **Warmup info** (optional second field): `<br>`-separated warmup sets

### Warmup Set Format

```
WU<N> · <Weight> kg · <Reps> reps
```

Weight can be:
- Standard: `37,5` (European decimal)
- Bodyweight-plus: `+0` (bodyweight only) or `+35` (bodyweight + 35kg)

### Column Header Row

```
#;KG;REPS;RIR
```

Always exactly this. Appears before each exercise's set data.

### Set Data Rows

```
<SetNum>;<Weight>;<Reps>;<RIR>
```

Examples:
```
1;115;8;1
2;102,5;6;0
1;+35;10;0
```

Fields:
- **Set number**: Sequential integer (1-based)
- **Weight (KG)**: European decimal. Prefix `+` means bodyweight-plus.
- **Reps**: Integer
- **RIR** (Reps in Reserve): Integer or European decimal (e.g. `0,5`)

### Session Separator

Blank line between sessions.

## Parsing Rules

1. Empty line → new session boundary
2. Line starts with quoted string containing `·` and ends with `hr"` → session header
3. Line starts with quoted string containing `·` and numbered exercise → exercise header
4. Line is `#;KG;REPS;RIR` → column header (skip)
5. Line matches `\d+;.*;.*;.*` → set data row
6. European decimals: replace `,` with `.` for parsing
7. Bodyweight-plus: `+N` → `is_bodyweight_plus=true`, `weight=N`
