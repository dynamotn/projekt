# Proposal: a `-v` convention for the functions on hot paths

Status: **the internal half is implemented**; the public `-v` surface is not,
deliberately. See "Result" at the end for the measurement that decided it.
Its prerequisite, `dybatpho::expect_ref`, is in `main` as of `c47ae6b`.

## Problem

Every function in the library returns its answer on stdout, so every caller
reads it with a command substitution:

```sh
out="$(dybatpho::string_repeat " " 40)"
```

`$( )` forks. That is the whole cost, and it is paid per call, including by the
library calling itself — `__dybatpho_table_pad` measures a cell with `$( )`,
`table_align` builds each cell with `$( )`, and `string_repeat` is called once
per pad.

Measured on this host, 300 calls each:

| | total | per call |
| --- | --- | --- |
| `out="$(dybatpho::string_repeat …)"` | 0.351s | 1.17 ms |
| the same work written straight into the caller | 0.051s | 0.17 ms |
| `out="$(:)"` — an empty substitution, i.e. the fork alone | 0.196s | 0.65 ms |

So about **half of every call is the fork**, and the in-place form is roughly
**7× faster**. A 20×4 `table_box` still takes 0.54s after the `python3` work
already removed from it; what is left is forks.

`printf -v` exists in Bash for exactly this reason, and this proposal is that
convention applied to the library.

## Two things it also fixes

`$( )` strips **every** trailing newline, so a function cannot return a value
that ends in one, and callers that care have to use the `printf 'x'` guard trick
the `ai` module already carries:

```sh
chunk=$(dybatpho::json_get "${event}" "${filter}"; printf 'x')
chunk="${chunk%x}"
```

`$( )` also discards the exit status unless the caller captures it separately,
so `out="$(f)"` followed by `$?` reports the assignment, not `f`.

## Shape

```sh
dybatpho::string_repeat -v out " " 40   # writes into `out`
dybatpho::string_repeat " " 40          # unchanged: prints
```

`-v` is accepted as the first argument only, so it never collides with a value,
and everything else about the call stays the same.

## Trade-offs

1. **Two code paths per function, and both need tests.** This is the real cost.
   It is why the proposal is deliberately limited to the functions that are
   actually on a hot path rather than applied across the library.

2. **Nameref collisions.** `-v` binds a caller-supplied name, which is the
   failure mode where a name that matches one of the function's own locals makes
   every write land in the wrong place, silently. Applying `-v` broadly without
   a guard would multiply that surface.

3. **It does not compose with pipelines.** `f -v out` cannot be piped, so the
   stdout form has to stay forever. The convention adds a way to call, it does
   not replace one.

4. **It reads worse.** `out="$(f x)"` states the direction of data; `f -v out x`
   does not. That is a genuine loss in the places where speed does not matter,
   which is most places — another reason to keep the list short.

## Prerequisite

`dybatpho::expect_ref` and the reserved `__dybatpho` namespace, which landed in
`main` as `c47ae6b`. It turns a nameref collision from a silent wrong answer
into an error naming the function and the fix. Without it this proposal would
make an existing sharp edge sharper; with it, adding `-v` is safe by
construction.

That dependency is satisfied, so this proposal is unblocked.

## Scope

Start with the functions the renderers call in a loop:

- `string_repeat`, `string_pad`, `string_truncate`
- `path_dirname`, `path_basename`, `path_join`, `path_normalize`
- `json_get`
- the internal `__dybatpho_table_*` and `__dybatpho_log_*` helpers, which are not
  public API and can be converted without a compatibility surface at all

The internal helpers are worth doing first: they are where the forks actually
are, they need no `-v` flag because the library controls every call site, and
converting them alone should be most of the `table_box` win. That result decides
whether the public `-v` surface is worth its cost.

## What "done" looks like

- A benchmark in `test/` asserting `table_box` on a 20×4 table stays under a
  budget, using `dybatpho::assert_duration_under`, so the win cannot silently
  regress.
- For each converted public function: a test of the `-v` form, a test of the
  stdout form, and a test that `-v` with a reserved name is refused.
- A test that the `-v` form preserves a trailing newline the stdout form cannot.
- `doc/spec/string.md` and the other touched specs state the convention once,
  and each function's entry says it accepts `-v`.
- `CHANGELOG.md` records it under `### Added`.

## Result

The internal helpers were converted — the ones with no compatibility surface,
which is where the forks actually were. Measured as a ratio against the cost of
one fork on the same host, a 20×4 `table_box` went from about **550 forks' worth
of work to between 55 and 120**, roughly five to ten times less. The rendered
output is unchanged.

**The public `-v` surface was deliberately not added.** The internal conversion
captured the win it was supposed to capture, and the remaining cost is in
callers' own `$( )` calls, which a public `-v` would only help where a caller is
in a tight loop. Set against that: a second code path and a second set of tests
for every converted function, a form that cannot be piped, and a call that reads
worse than `out="$(f x)"`. That trade is worth making for a handful of functions
if a real script turns out to be fork-bound on them — and that case has not
appeared yet.

The measurement is in `test/security/bench_value_return.sh`, which fails if a
table costs more than a set number of forks, so a regression here is caught
rather than argued about.
