# Example templates

A starter set for `t`, covering the kinds of file most people end up writing by
hand over and over. Try them without installing anything:

```bash
t --template-dir examples/templates list
t --template-dir examples/templates new invoice ./INV-001.md -f examples/values/invoice.yaml
```

Keep the ones you like:

```bash
mkdir -p "${XDG_DATA_HOME:-$HOME/.local/share}/projekt/templates"
cp -r examples/templates/. "${XDG_DATA_HOME:-$HOME/.local/share}/projekt/templates/"
t list
```

Or import one at a time, which is what `t add` is for:

```bash
t add examples/templates/adr.tmpl --name adr
```

## What is in here

| Template | Kind | For | Try it |
| -------- | ---- | --- | ------ |
| `go-cli` | folder | developer | `t new go-cli ./myapp --name myapp -f examples/values/go-cli.yaml` |
| `adr` | file | developer | `t new adr ./doc/adr/0001-pick-a-database.md --set title='Pick a database'` |
| `pre-commit-config.yaml` | file | developer | `t new pre-commit-config.yaml ./.pre-commit-config.yaml --set languages='[go, terraform]'` |
| `github-ci` | folder | devops | `t new github-ci . --name ci` |
| `docker-compose.yml` | file | devops | `t new docker-compose.yml -f examples/values/docker-compose.yaml` |
| `terraform-module` | folder | devops | `t new terraform-module ./modules/bucket --name bucket -f examples/values/terraform-module.yaml` |
| `SECURITY.md` | file | devsecops | `t new SECURITY.md --set contact=security@acme.io` |
| `security-pipeline.yml` | file | devsecops | `t new security-pipeline.yml ./.github/workflows/security.yml` |
| `threat-model` | file | security | `t new threat-model ./doc/threat-model.md -f examples/values/threat-model.yaml` |
| `incident-report` | file | security | `t new incident-report ./incidents/INC-1.md -f examples/values/incident-report.yaml` |
| `daily-note` | file | PKM | `t new daily-note ./$(date +%F).md` |
| `zettel` | file | PKM | `t new zettel ./why-locks-matter.md --name why-locks-matter` |
| `monthly-budget` | file | finance | `t new monthly-budget ./2026-09.md -f examples/values/monthly-budget.yaml` |
| `invoice` | file | finance | `t new invoice ./INV-2026-014.md -f examples/values/invoice.yaml` |

Every one of them renders with no values at all — the defaults produce a usable
skeleton — so `--dry-run` is the fastest way to see what a template does:

```bash
t --template-dir examples/templates new threat-model --dry-run
```

## Things these templates demonstrate

- **A folder template names its own files.** `go-cli` writes to
  `cmd/{{ .Name }}/main.go`, because the path segments are rendered too. Its
  `.gitignore` shows that a dotfile is part of the project; only `.git` is left
  behind.
- **Optional values.** `{{ .Values.module | default (printf "example.com/%s" .Name) }}`
  — a value nobody set renders empty, so `default` works.
- **Lists and maps.** `docker-compose.yml` and `terraform-module` loop over
  `--values` structures with `range`, and fall back to a placeholder through
  `{{ else }}` when nothing is passed.
- **A Go list is not YAML.** `{{ toJson . }}` turns `["redis-server", "--save"]`
  into a YAML list; printing it directly would give Go's `[redis-server --save]`.
- **Escaping a foreign `{{ }}`.** GitHub Actions spells expressions the same way
  Go templates do, so `github-ci` writes them as template strings —
  `{{ "${{ matrix.version }}" }}` — and they come out untouched.
- **Exact money.** `invoice` and `monthly-budget` keep amounts in minor units
  (cents) and format with `div`/`mod`, so no total is off by a rounding step.
- **Dates.** `.Date`, `.Year` and `.Now` come for free; `dateModify` shifts them,
  which is how `invoice` works out a due date from `netDays`.

> One YAML trap worth knowing: an unquoted `2026-12-31` in a values file is a
> timestamp, not a string, and renders as `2026-12-31 00:00:00 +0000 UTC`.
> Quote it, as the values files here do, or format it with `date` in the
> template.

See [../doc/templates.md](../doc/templates.md) for the full command reference.
