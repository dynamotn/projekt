#!/usr/bin/env bash
# @file safety_ops.sh
# @brief Example showing guards for destructive operations
# @description Demonstrates dybatpho::confirm, assert_safe_path, safe_rm,
#              safe_overwrite, safe_copy, safe_move, safe_extract, and
#              safe_system in a non-interactive release-cleanup workflow
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPTDIR}/../init.sh" --modules archive safety

dybatpho::register_common_handlers

# The whole demo runs unattended, so confirmations must be refused unless an
# operation explicitly passes --force.
export DYBATPHO_INTERACTIVE=false

WORKDIR="$(mktemp -d)"
# Nothing in this demo may touch a path outside the scratch directory.
export DYBATPHO_SAFE_ROOTS="${WORKDIR}"
dybatpho::cleanup_file_on_exit "${WORKDIR}"

# --- validating paths --------------------------------------------------------

function _demo_assert_safe_path {
  dybatpho::header "SAFE PATHS"
  dybatpho::info "Release dir resolves to: $(dybatpho::assert_safe_path "${WORKDIR}/release/../release")"

  if ! (dybatpho::assert_safe_path "/usr" > /dev/null) 2> /dev/null; then
    dybatpho::warn "A protected system path was rejected (expected)"
  fi
  if ! (dybatpho::assert_safe_path "${WORKDIR}/../escape" > /dev/null) 2> /dev/null; then
    dybatpho::warn "A path outside DYBATPHO_SAFE_ROOTS was rejected (expected)"
  fi
}

# --- removing files ----------------------------------------------------------

function _demo_safe_rm {
  dybatpho::header "SAFE RM"
  mkdir -p "${WORKDIR}/cache/objects"
  printf 'cached\n' > "${WORKDIR}/cache/objects/blob"

  if ! dybatpho::safe_rm "${WORKDIR}/cache/objects/blob" 2> /dev/null; then
    dybatpho::warn "Unattended removal without --force was refused (expected)"
  fi

  if ! (dybatpho::safe_rm --force "${WORKDIR}/cache") 2> /dev/null; then
    dybatpho::warn "Removing a directory without --recursive was refused (expected)"
  fi

  DRY_RUN=true dybatpho::safe_rm --force --recursive "${WORKDIR}/cache"
  dybatpho::info "DRY_RUN kept the directory: $(dybatpho::is dir "${WORKDIR}/cache" && echo yes || echo no)"

  dybatpho::safe_rm --force --recursive "${WORKDIR}/cache"
  dybatpho::success "Cache directory removed"
}

# --- overwriting files -------------------------------------------------------

function _demo_safe_overwrite {
  dybatpho::header "SAFE OVERWRITE"
  local config="${WORKDIR}/app.conf"
  printf 'mode=old\n' > "${config}"

  if ! dybatpho::safe_overwrite "${config}" 2> /dev/null; then
    dybatpho::warn "Unattended overwrite without --force was refused (expected)"
  fi

  dybatpho::safe_overwrite --force --backup "${config}"
  printf 'mode=new\n' > "${config}"
  dybatpho::info "Current: $(cat "${config}")"
  dybatpho::info "Backup:  $(cat "${config}.bak")"
}

# --- copying and moving ------------------------------------------------------

function _demo_safe_copy_move {
  dybatpho::header "SAFE COPY / MOVE"
  mkdir -p "${WORKDIR}/release"
  dybatpho::safe_copy --force "${WORKDIR}/app.conf" "${WORKDIR}/release"
  dybatpho::info "Copied into the release dir: $(dybatpho::path_basename "${WORKDIR}/release/app.conf")"

  dybatpho::safe_move --force "${WORKDIR}/app.conf.bak" "${WORKDIR}/release/backups/app.conf.bak"
  dybatpho::success "Backup moved, missing parent directories created"
}

# --- extracting archives -----------------------------------------------------

function _demo_safe_extract {
  dybatpho::header "SAFE EXTRACT"
  mkdir -p "${WORKDIR}/payload/bundle/nested" "${WORKDIR}/unpacked"
  printf 'artifact\n' > "${WORKDIR}/payload/bundle/nested/file.txt"
  dybatpho::archive_create "${WORKDIR}/payload/bundle" "${WORKDIR}/bundle.tar.gz"

  dybatpho::safe_extract --force "${WORKDIR}/bundle.tar.gz" "${WORKDIR}/unpacked" 1
  dybatpho::info "Extracted: $(cat "${WORKDIR}/unpacked/nested/file.txt")"

  # Craft an archive whose entry escapes the destination, like a hostile release
  # tarball would.
  printf 'owned\n' > "${WORKDIR}/payload/victim.txt"
  (
    cd "${WORKDIR}/payload/bundle"
    tar -czf "${WORKDIR}/evil.tar.gz" -P ../victim.txt 2> /dev/null
  )
  dybatpho::info "Unsafe entries: $(dybatpho::archive_unsafe_entries "${WORKDIR}/evil.tar.gz" 2> /dev/null)"
  if ! (dybatpho::safe_extract --force "${WORKDIR}/evil.tar.gz" "${WORKDIR}/unpacked") 2> /dev/null; then
    dybatpho::warn "A path-traversal archive was rejected (expected)"
  fi
}

# --- system changes ----------------------------------------------------------

function _demo_safe_system {
  dybatpho::header "SAFE SYSTEM CHANGE"
  if ! dybatpho::safe_system "Restart the app service" -- touch "${WORKDIR}/restarted" 2> /dev/null; then
    dybatpho::warn "Unattended system change without --force was skipped (expected)"
  fi

  DRY_RUN=true dybatpho::safe_system --force "Restart the app service" -- touch "${WORKDIR}/restarted"
  dybatpho::safe_system --force "Restart the app service" -- touch "${WORKDIR}/restarted"
  dybatpho::success "System change applied: $(dybatpho::is file "${WORKDIR}/restarted" && echo yes || echo no)"
}

# --- main --------------------------------------------------------------------

function _main {
  _demo_assert_safe_path
  _demo_safe_rm
  _demo_safe_overwrite
  _demo_safe_copy_move
  _demo_safe_extract
  _demo_safe_system
  dybatpho::success "Safety operations demo complete"
}

_main "$@"
