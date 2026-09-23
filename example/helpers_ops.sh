#!/usr/bin/env bash
# @file helpers_ops.sh
# @brief Example showing dependency checks, conditions, defaults, and retry.
# shellcheck disable=SC2034 # the DYBATPHO_RETRY_* values are read by the helpers module
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

# How long a retry waits is configurable, and the delay grows exponentially from
# the base up to the cap: 2, 4, 8, 16, then 30 with these defaults.
dybatpho::print "backoff with the defaults:"
for attempt in 1 2 3 4 5 6; do
  dybatpho::print "  attempt ${attempt}: $(__dybatpho_helpers_backoff "${attempt}")s"
done

# Jitter spreads retries out when several machines are waiting on the same
# failing dependency, so they do not all come back at the same instant.
DYBATPHO_RETRY_BASE_DELAY=1
DYBATPHO_RETRY_MAX_DELAY=10
DYBATPHO_RETRY_JITTER=true
dybatpho::print "same attempt, three draws with jitter on:"
dybatpho::print "  $(__dybatpho_helpers_backoff 4)s $(__dybatpho_helpers_backoff 4)s $(__dybatpho_helpers_backoff 4)s"

dybatpho::success "Helper demo complete for ${EXAMPLE_NAME}"
