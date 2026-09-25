#!/usr/bin/env bash
# PoC / regression -- dybatpho::table_csv splits on every comma, including the
# ones inside a quoted field, so a row of real CSV silently gained columns and
# no longer matched its header.
#
# This was never a broken promise: doc/spec/table.md scopes the helper as a
# "CSV convenience wrapper" over comma-delimited data, not as an RFC 4180
# parser. It was the name promising more than the contract, with a data-loss
# outcome -- and nothing said so.
#
# The fix is not a parser. It refuses input it cannot read, which turns silent
# corruption into an error that names the limitation. This checks that, that
# the escape hatch still works, and that ordinary input is not caught by it.
set -uo pipefail

ROOT="${1:?usage: csv_quoted_field.sh <repo-root>}"

export LOG_LEVEL=fatal
# shellcheck disable=SC1090
source "${ROOT}/init.sh" --modules table

status=0
QUOTED="$(printf 'name,note\n"Doe, John",ok')"

echo "== a quoted field containing a comma =="
if output="$(dybatpho::table_csv "${QUOTED}" markdown 2>&1)"; then
  echo "    rendered anyway:"
  printf '%s\n' "${output}" | sed 's/^/      /'
  echo "    WRONG: the quoted comma became a column separator"
  status=1
else
  echo "    refused, as it should be:"
  printf '%s\n' "${output}" | sed 's/\x1b\[[0-9;]*m//g' | sed 's/^/      /' | head -3
fi

echo
echo "== a quoted field containing an escaped quote =="
if output="$(dybatpho::table_csv "$(printf 'a,b\n"He said ""hi""",x')" markdown 2>&1)"; then
  echo "    WRONG: rendered input it cannot parse"
  status=1
else
  echo "    refused"
fi

echo
echo "== the escape hatch still splits, for data known to carry no quoting =="
if output="$(DYBATPHO_TABLE_CSV_STRICT=false dybatpho::table_csv "${QUOTED}" markdown 2>&1)"; then
  echo "    rendered:"
  printf '%s\n' "${output}" | sed 's/^/      /'
else
  echo "    WRONG: the opt-out did not work"
  status=1
fi

echo
echo "== a quote that is not a field boundary is data, not CSV quoting =="
if output="$(dybatpho::table_csv "$(printf 'size,note\n5\" pipe,ok')" markdown 2>&1)"; then
  echo "    rendered:"
  printf '%s\n' "${output}" | sed 's/^/      /'
else
  echo "    WRONG: refused ordinary data containing a quote character"
  printf '%s\n' "${output}" | sed 's/\x1b\[[0-9;]*m//g' | sed 's/^/      /' | head -2
  status=1
fi

echo
echo "== the delimiter-convenience case this helper exists for =="
if output="$(printf 'web 1/1 Running\napi 1/1 Running\n' | tr -s ' ' ',' | dybatpho::table_csv - markdown 2>&1)"; then
  echo "    rendered:"
  printf '%s\n' "${output}" | sed 's/^/      /'
else
  echo "    WRONG: refused the case the helper is documented for"
  status=1
fi

echo
if ((status == 0)); then
  echo "OK: unreadable input is refused, readable input still renders"
  exit 0
fi
echo "CONFIRMED: see above"
exit 1
