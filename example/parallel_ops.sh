#!/usr/bin/env bash
# @file parallel_ops.sh
# @brief Example showing bounded concurrency with ordered output
# @description Demonstrates dybatpho::parallel_map, parallel_run,
#   parallel_status, parallel_count, parallel_failed, fail-fast, and DRY_RUN
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
. "${SCRIPTDIR}/../init.sh" --modules parallel

dybatpho::register_common_handlers

HOSTS=(web-01 web-02 db-01 cache-01 worker-01 worker-02)

function _check_host {
  local host="$1"
  # Pretend work, long enough that the jobs really do overlap.
  sleep "0.$((RANDOM % 3 + 1))"
  dybatpho::print "  ${host}: reachable"
  # One host is deliberately unhealthy, to show per-job exit codes.
  [[ "${host}" == "db-01" ]] && return 1
  return 0
}

function _demo_map {
  dybatpho::header "ONE COMMAND OVER A LIST"
  dybatpho::info "Checking ${#HOSTS[@]} hosts, 3 at a time"

  # Called directly rather than through a substitution: the per-job exit codes
  # are kept in this shell, and a subshell would take them with it.
  dybatpho::parallel_map 3 _check_host "${HOSTS[@]}" || true

  dybatpho::info "Jobs: $(dybatpho::parallel_count), failed: $(dybatpho::parallel_failed)"
  local index
  for ((index = 0; index < $(dybatpho::parallel_count); index++)); do
    dybatpho::print "  $(printf '%-10s' "${HOSTS[index]}") exit $(dybatpho::parallel_status "${index}")"
  done
}

function _demo_ordering {
  dybatpho::header "OUTPUT STAYS READABLE"
  dybatpho::info "Jobs overlap, but each one's lines are replayed together"

  function _noisy {
    printf '  [%s] starting\n' "$1"
    sleep "0.$((RANDOM % 3))"
    printf '  [%s] finished\n' "$1"
  }
  dybatpho::parallel_map 4 _noisy alpha bravo charlie delta
}

function _demo_run {
  dybatpho::header "DIFFERENT COMMANDS AT ONCE"
  # `parallel_run` suits jobs that are not the same command over a list.
  dybatpho::parallel_run 3 \
    "printf '  linting...\n'; sleep 0.1; printf '  lint done\n'" \
    "printf '  testing...\n'; sleep 0.2; printf '  tests done\n'" \
    "printf '  building...\n'; sleep 0.1; printf '  build done\n'"
}

function _demo_failfast {
  dybatpho::header "FAIL FAST"
  dybatpho::info "With fail-fast on, the jobs after a failure are never started"

  function _step {
    dybatpho::print "  running ${1}"
    [[ "$1" == "step-2" ]] && return 1
    return 0
  }
  DYBATPHO_PARALLEL_FAILFAST=true
  dybatpho::parallel_map 1 _step step-1 step-2 step-3 step-4 || true
  DYBATPHO_PARALLEL_FAILFAST=false

  local index
  for ((index = 0; index < $(dybatpho::parallel_count); index++)); do
    dybatpho::print "  job ${index} -> $(dybatpho::parallel_status "${index}")"
  done
  dybatpho::info "A skipped job never ran, so it counts as neither pass nor fail"
}

function _demo_dry_run {
  dybatpho::header "DRY RUN"
  DRY_RUN=true dybatpho::parallel_map 2 _check_host web-01 db-01
  dybatpho::info "Nothing was contacted"
}

function _main {
  _demo_map
  _demo_ordering
  _demo_run
  _demo_failfast
  _demo_dry_run
  dybatpho::success "Parallel demo complete"
}

_main "$@"
