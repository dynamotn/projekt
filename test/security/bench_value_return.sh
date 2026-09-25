#!/usr/bin/env bash
# Benchmark -- what a command substitution costs, and how much of drawing a
# table is spent on them.
#
# Wall-clock numbers are meaningless on a loaded machine, so the figure that
# matters here is the *ratio*: how many forks' worth of time a `table_box`
# takes. That is stable whether the host is idle or busy, which makes it usable
# as a regression threshold.
set -uo pipefail

ROOT="${1:?usage: bench_value_return.sh <repo-root>}"
# How many forks a 20x4 table may cost before this is considered a regression.
BUDGET="${2:-250}"

export LOG_LEVEL=fatal
# shellcheck disable=SC1090
source "${ROOT}/init.sh" --modules table string

N=300

elapsed_ms() {
  awk -v a="$1" -v b="$2" 'BEGIN{printf "%.0f", (b - a) * 1000}'
}

# ---------------------------------------------------------------- fork cost
t0="${EPOCHREALTIME}"
for ((i = 0; i < N; i++)); do
  # The substitution is what is being timed; the value is not wanted.
  : "$(:)"
done
t1="${EPOCHREALTIME}"
fork_total="$(elapsed_ms "${t0}" "${t1}")"
fork_us="$(awk -v t="${fork_total}" -v n="${N}" 'BEGIN{printf "%.0f", t * 1000 / n}')"

echo "== the cost of one fork on this host, right now =="
printf '  %s empty command substitutions: %sms (%sus each)\n' "${N}" "${fork_total}" "${fork_us}"

# --------------------------------------------------- value return vs $( )
via_subshell() {
  local i out=""
  for ((i = 0; i < N; i++)); do out="$(dybatpho::string_repeat "-" 40)"; done
  printf '%s' "${out}" > /dev/null
}
repeat_in_place() {
  local -n __dybatpho_out="$1"
  local token="$2" count="$3" i
  __dybatpho_out=""
  for ((i = 0; i < count; i++)); do __dybatpho_out+="${token}"; done
}
via_nameref() {
  local i out=""
  for ((i = 0; i < N; i++)); do repeat_in_place out "-" 40; done
  printf '%s' "${out}" > /dev/null
}

t0="${EPOCHREALTIME}"
via_subshell
t1="${EPOCHREALTIME}"
sub_ms="$(elapsed_ms "${t0}" "${t1}")"

t0="${EPOCHREALTIME}"
via_nameref
t1="${EPOCHREALTIME}"
ref_ms="$(elapsed_ms "${t0}" "${t1}")"

echo
echo "== returning a value, ${N} calls =="
printf '  through a command substitution: %sms\n' "${sub_ms}"
printf '  into a named variable:        %sms\n' "${ref_ms}"
awk -v a="${sub_ms}" -v b="${ref_ms}" \
  'BEGIN{ if (b > 0) printf "  %.1fx faster\n", a / b }'

# ------------------------------------------------------------- table_box
data=""
for ((i = 1; i <= 20; i++)); do data+="row${i}|value${i}|third${i}|fourth${i}"$'\n'; done
t0="${EPOCHREALTIME}"
dybatpho::table_box "${data%$'\n'}" '|' > /dev/null
t1="${EPOCHREALTIME}"
table_ms="$(elapsed_ms "${t0}" "${t1}")"

forks="$(awk -v t="${table_ms}" -v f="${fork_us}" \
  'BEGIN{ if (f > 0) printf "%.0f", t * 1000 / f; else print 0 }')"

echo
echo "== a 20x4 table_box =="
printf '  wall clock:      %sms\n' "${table_ms}"
printf '  in fork-equivalents: %s  (budget %s)\n' "${forks}" "${BUDGET}"

echo
if ((forks > BUDGET)); then
  echo "REGRESSION: drawing a table costs more than ${BUDGET} forks' worth of work"
  exit 1
fi
echo "OK: within budget"
exit 0
