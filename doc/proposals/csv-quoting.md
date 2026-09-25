# Proposal: stop `table_csv` mis-reading real CSV

Status: proposed, not implemented. The evidence below is reproducible today.

## Problem

`dybatpho::table_csv` splits rows on every comma:

```sh
plain) dybatpho::table_align "${input}" "," "${align_spec}" 2 ;;
```

So a field that legitimately contains a comma, quoted the way CSV requires,
becomes two columns:

```
input row:  "Doe, John",ok          two fields
rendered:   | "Doe | John" | ok |   three cells, under a two-column header
```

Quotes are also passed through as text rather than decoded, so
`"He said ""hi"""` renders with its quoting intact instead of as
`He said "hi"`.

Nothing reports a problem. The table is simply wrong, and wrong in a way that
looks plausible enough to be read as data.

## Is this a bug?

Strictly, no, and the proposal should not pretend otherwise. `doc/spec/table.md`
scopes the helper as a "CSV convenience wrapper" over "comma-delimited" input —
it never claims RFC 4180. The documented example is

```sh
kubectl get pods --no-headers | tr -s ' ' ',' | dybatpho::table_csv - box
```

which is exactly the delimiter-convenience case it was written for.

The problem is the name. `table_csv` is what a caller reaches for when they have
a CSV file, and CSV files contain quoted commas — addresses, names, free-text
notes, anything exported from a spreadsheet. The helper does something
reasonable for its actual contract and something silently destructive for the
contract its name implies.

## Evidence

```
$ bash test/security/csv_quoted_field.sh "$PWD"
== a quoted field containing a comma ==
    input row: "Doe, John",ok        (2 fields: 'Doe, John' and 'ok')
      | name | note  |    |
      | ---- | ----- | --- |
      | "Doe | John" | ok |
    header columns: 2, data row cells: 3
    WRONG: the quoted comma became a column separator
```

## Options

### A. Parse RFC 4180 properly

Add a real field splitter: quoted fields, `""` as an escaped quote, embedded
commas and newlines, and use it from `table_csv`.

- The helper then means what its name says.
- Embedded newlines are the awkward part: a single record can span lines, so the
  "one row per line" assumption that `__dybatpho_text_read_lines` and every
  renderer are built on stops holding. Handling that honestly means parsing
  records before splitting rows, which is a real change to the module's shape,
  not a new function next to the old one.
- Pure Bash character-at-a-time parsing is slow, and the table renderers are
  already the slowest thing in the library.

### B. Rename, and say what it does

Keep the behaviour, rename to `table_delimited` (keeping `table_csv` as a
deprecated alias), and make the documentation say "splits on the delimiter; does
not understand quoting".

- Honest and nearly free.
- Does not help the caller who has an actual CSV; it only stops surprising them.

### C. Detect and refuse

Keep splitting on commas, but when a row contains a `"` at a field boundary,
stop with a message naming the limitation.

- Turns silent corruption into a loud error, which is the property that matters
  most.
- Cheap: one check per row, no parser.
- Rejects input the current behaviour handles "well enough" for someone who
  knows their data has no quoting, so it needs an opt-out.

## Recommendation

**C now, A later, and B alongside either.**

The immediate harm is that the output is wrong without saying so; C removes
exactly that, for a few lines. A is the right end state but it changes the
module's row model and deserves its own decision rather than being smuggled in
under a bug fix.

Concretely, first change:

- `table_csv` refuses a row whose fields are quoted, naming
  `DYBATPHO_TABLE_CSV_STRICT=false` as the way to get the old behaviour.
- The documentation and spec say "delimiter-split, not RFC 4180".

## What "done" looks like

- `test/security/csv_quoted_field.sh` exits 0 — either because the fields parse
  correctly (A) or because the helper refuses and says why (C).
- A test covers a quoted comma, an escaped quote, and, if A is taken, a field
  with an embedded newline.
- `doc/spec/table.md` states the contract explicitly and gains `IT-` entries.
- `CHANGELOG.md` records it under `### Changed`, since input that used to render
  now stops.
