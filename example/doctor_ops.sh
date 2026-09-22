#!/usr/bin/env bash
# @file doctor_ops.sh
# @brief Example showing the environment report a script can run before it works
# @description Demonstrates dybatpho::version, dybatpho::doctor with a module
#   scope, --json and --quiet, dybatpho::doctor_requirements, and
#   dybatpho::doctor_bash_supported
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
. "${SCRIPTDIR}/../init.sh" --modules doctor json archive git

dybatpho::register_common_handlers

function _demo_version {
  dybatpho::header "WHICH LIBRARY AM I RUNNING?"
  # Available from `init.sh` alone, so a script can report it before deciding
  # whether it has the functions it needs.
  dybatpho::info "dybatpho $(dybatpho::version)"
}

function _demo_report {
  dybatpho::header "WHAT THIS SHELL LOADED, AND WHAT IT NEEDS"
  # No argument means the modules this script asked `init.sh` for. The report
  # returns non-zero when a required dependency is missing, and this example
  # keeps going either way so the rest of it still runs on a bare machine.
  dybatpho::doctor || dybatpho::warn "Something required is missing, see the report above"
}

function _demo_scope {
  dybatpho::header "CHECKING A MODULE A SCRIPT HAS NOT LOADED YET"
  # Useful before `dybatpho::load`: ask whether the environment can support a
  # module before the script commits to it.
  dybatpho::doctor --modules "archive,git" || true
}

function _demo_requirements {
  dybatpho::header "WHAT ONE MODULE CALLS"
  local kind spec
  for kind in required optional; do
    dybatpho::info "${kind} for archive:"
    while read -r spec; do
      dybatpho::print "  ${spec}"
    done < <(dybatpho::doctor_requirements archive "${kind}")
  done
}

function _demo_quiet {
  dybatpho::header "A GATE FOR CI"
  # `--quiet` prints nothing and answers through the exit code, which is what a
  # pipeline step wants.
  if dybatpho::doctor --modules json --quiet; then
    dybatpho::success "The json module can run here"
  else
    dybatpho::warn "The json module is missing a required dependency"
  fi
}

function _demo_json {
  dybatpho::header "MACHINE-READABLE REPORT"
  local report
  # The report is built without `jq`, on purpose: a diagnostic that needs a tool
  # the user may be missing is of no use.
  report="$(dybatpho::doctor --modules "json,archive" --json || true)"
  if dybatpho::is command jq; then
    dybatpho::print "Missing dependencies, as jq sees them:"
    printf '%s' "${report}" \
      | jq -r '.dependencies[] | select(.status == "missing") | "  \(.module): \(.dependency) (\(.kind))"'
    dybatpho::print "Everything required is present: $(printf '%s' "${report}" | jq -r '.ok')"
  else
    dybatpho::print "${report:0:120}..."
  fi
}

function _demo_bash {
  dybatpho::header "IS THIS SHELL NEW ENOUGH?"
  if dybatpho::doctor_bash_supported; then
    dybatpho::success "Bash ${BASH_VERSION} is at least ${DYBATPHO_BASH_MINIMUM}"
  else
    dybatpho::error "Bash ${BASH_VERSION} is older than ${DYBATPHO_BASH_MINIMUM}"
  fi
}

_demo_version
_demo_report
_demo_scope
_demo_requirements
_demo_quiet
_demo_json
_demo_bash

dybatpho::success "Doctor example finished"
