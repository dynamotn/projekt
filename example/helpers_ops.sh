#!/usr/bin/env bash
# @file helpers_ops.sh
# @brief Example showing dependency checks, conditions, defaults, and retry.
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
. "${SCRIPTDIR}/../init.sh"

dybatpho::require bash
# A version range turns "is it installed" into "is it usable". Ranges need the
# optional `semver` module, and `require` says so rather than letting the range
# pass unchecked.
dybatpho::load semver
dybatpho::require bash '>=4.3'
dybatpho::default_env EXAMPLE_NAME "dybatpho"

if dybatpho::is command curl; then
  dybatpho::info "curl is available"
fi

api_url="$(dybatpho::coalesce "${EXAMPLE_URL:-}" "https://example.com")"
dybatpho::print "target: ${api_url}"

attempt_file="${TMPDIR:-/tmp}/dybatpho-helper-example-${BASHPID}"
trap 'rm -f -- "${attempt_file}"' EXIT
printf 'ready\n' > "${attempt_file}"
dybatpho::retry 2 "grep -q ready '${attempt_file}'" "readiness check"
dybatpho::success "Helper demo complete for ${EXAMPLE_NAME}"
