#!/usr/bin/env bash
# @file cache_ops.sh
# @brief Remember a slow answer on disk until it goes stale
# @description
#   Runs entirely offline: the "slow" command is a local function that records
#   how many times it was actually called, which is the only way to show that a
#   cache did anything.
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPTDIR}/../init.sh" --modules cache

dybatpho::register_common_handlers

# Keep the demonstration out of the real cache directory. The helper sets the
# named variable and registers the directory for cleanup when this script ends.
dybatpho::create_temp_dir CACHE_ROOT "cache-example"
export DYBATPHO_CACHE_DIR="${CACHE_ROOT}/cache"
export DYBATPHO_CACHE_NAMESPACE="example"

CALLS_FILE="${CACHE_ROOT}/calls"
printf '0\n' > "${CALLS_FILE}"

# @description Stand in for the slow thing a real script would cache: an API
#   listing, a dependency resolution, a probe across a fleet.
function _expensive_listing {
  printf '%s\n' "$(($(cat "${CALLS_FILE}") + 1))" > "${CALLS_FILE}"
  printf 'release-1.0\nrelease-1.1\n'
}

function _calls_so_far {
  cat "${CALLS_FILE}"
}

# @description The whole module in one call: ask once, reuse until it expires.
function _demo_run {
  dybatpho::header "MEMOIZING A COMMAND"
  local first second
  first="$(dybatpho::cache_run releases 3600 -- _expensive_listing)"
  second="$(dybatpho::cache_run releases 3600 -- _expensive_listing)"
  dybatpho::print "  first call:  $(printf '%s' "${first}" | tr '\n' ' ')"
  dybatpho::print "  second call: $(printf '%s' "${second}" | tr '\n' ' ')"
  dybatpho::print "  the command actually ran $(_calls_so_far) time(s)"

  # A time to live of zero makes nothing fresh, which is how a script offers a
  # `--refresh` flag without deleting anything.
  dybatpho::cache_run releases 0 -- _expensive_listing > /dev/null
  dybatpho::print "  after a forced refresh it ran $(_calls_so_far) time(s)"
}

# @description A command that fails is never stored, so the next call asks
#   again rather than repeating a remembered error for an hour.
function _demo_failure_is_not_remembered {
  dybatpho::header "A FAILURE IS NOT AN ANSWER"
  local status=0
  dybatpho::cache_run flaky 3600 -- false > /dev/null 2>&1 || status=$?
  dybatpho::print "  the command exited ${status}, and its own status came back"
  if dybatpho::cache_has flaky 3600; then
    dybatpho::warn "  it was stored, which it should not have been"
  else
    dybatpho::print "  nothing was stored, so the next call asks again"
  fi
}

# @description Store and read an entry directly, for the times the value does
#   not come from running a command.
function _demo_direct {
  dybatpho::header "STORING A VALUE DIRECTLY"
  printf 'v4.53.3\n' | dybatpho::cache_set resolved-version
  dybatpho::print "  stored at $(dybatpho::cache_path resolved-version)"
  dybatpho::print "  reads back as $(dybatpho::cache_get resolved-version 3600)"

  # A key becomes a file name, so anything that is not already a short name
  # goes through the hash first.
  local key
  key="$(dybatpho::cache_key "https://example.com/api/things?page=2")"
  printf 'page two\n' | dybatpho::cache_set "${key}"
  dybatpho::print "  a URL keys as ${key:0:16}..."
  dybatpho::print "  and reads back as $(dybatpho::cache_get "${key}" 3600)"
}

# @description Entries expire on their modification time, so an old one is not
#   used just because it is there.
function _demo_staleness {
  dybatpho::header "STALENESS"
  printf 'yesterday\n' | dybatpho::cache_set report
  # Backdate the entry rather than waiting an hour for it to age.
  touch -d '2 hours ago' "$(dybatpho::cache_path report)" 2> /dev/null \
    || touch -t 200001010000 "$(dybatpho::cache_path report)"
  if dybatpho::cache_has report 3600; then
    dybatpho::warn "  still counted as fresh"
  else
    dybatpho::print "  older than an hour, so it is not used"
  fi
  dybatpho::print "  but it is still there under a longer budget: $(dybatpho::cache_get report 999999999)"
}

# @description Namespaces keep unrelated caches from colliding on a key.
function _demo_namespaces {
  dybatpho::header "NAMESPACES"
  printf 'from github\n' | DYBATPHO_CACHE_NAMESPACE=github dybatpho::cache_set listing
  printf 'from gitlab\n' | DYBATPHO_CACHE_NAMESPACE=gitlab dybatpho::cache_set listing
  dybatpho::print "  github: $(DYBATPHO_CACHE_NAMESPACE=github dybatpho::cache_get listing 3600)"
  dybatpho::print "  gitlab: $(DYBATPHO_CACHE_NAMESPACE=gitlab dybatpho::cache_get listing 3600)"

  DYBATPHO_CACHE_NAMESPACE=github dybatpho::cache_clear
  if DYBATPHO_CACHE_NAMESPACE=github dybatpho::cache_has listing 3600; then
    dybatpho::warn "  github survived being cleared"
  else
    dybatpho::print "  github cleared, gitlab untouched: $(DYBATPHO_CACHE_NAMESPACE=gitlab dybatpho::cache_get listing 3600)"
  fi
}

function _main {
  _demo_run
  _demo_failure_is_not_remembered
  _demo_direct
  _demo_staleness
  _demo_namespaces
  dybatpho::cache_clear
  dybatpho::success "Cache operations demo complete"
}

_main "$@"
