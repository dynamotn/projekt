#!/usr/bin/env bash
# @file lock.sh
# @brief Utilities for process locking and coordination
# @description
#   This module provides a portable file lock (Linux/macOS) built on the
#   atomicity of `mkdir`, so it works the same way without depending on
#   `flock`, which isn't shipped by default on macOS.
#
#   A lock is a directory containing metadata about the process holding it
#   (pid, hostname, command, and acquisition time), which lets callers:
#
#   - prevent two runs of the same script from executing concurrently
#   - wait for a lock with a timeout instead of failing immediately
#   - inspect which process currently holds a lock
#   - detect and reclaim stale locks left behind by a dead process
# @usage
#   ### Prevent concurrent runs of the same script
#
#   ```bash
#   dybatpho::lock_acquire "$(basename "$0")" || dybatpho::die "Already running"
#   trap 'dybatpho::lock_release "$(basename "$0")"' EXIT
#   ```
#
#   ### Wait up to 30s for a lock, then run a command while holding it
#
#   ```bash
#   dybatpho::with_lock "deploy" 30 -- ./deploy.sh
#   ```
#
#   ### Inspect who is holding a lock
#
#   ```bash
#   dybatpho::lock_info "deploy"
#   ```
# @see
#   - `example/lock_ops.sh`
: "${DYBATPHO_DIR:?DYBATPHO_DIR must be set. Please source dybatpho/init.sh before other scripts from dybatpho.}"

# @env DYBATPHO_LOCK_DIR string Base directory used to resolve lock names into lock paths
DYBATPHO_LOCK_DIR="${DYBATPHO_LOCK_DIR:-${TMPDIR:-/tmp}}"
# @env DYBATPHO_LOCK_POLL_INTERVAL number Seconds to sleep between acquire attempts while waiting
DYBATPHO_LOCK_POLL_INTERVAL="${DYBATPHO_LOCK_POLL_INTERVAL:-1}"

#######################################
# @description Print the current host name using whichever mechanism is available.
#   Kept as the name a lock file is stamped with; the detection itself lives in
#   `dybatpho::hostname`.
# @stdout Host name reported by `hostname`, `uname -n`, the kernel, or the `HOSTNAME` env var
#######################################
function dybatpho::lock_hostname {
  dybatpho::hostname
}

#######################################
# @description Resolve a lock name or path into an absolute lock directory path.
# @arg $1 string Lock name (bare word) or an explicit absolute/relative path
# @stdout Absolute lock directory path, always suffixed with `.lock`
#######################################
function dybatpho::lock_path {
  local name
  dybatpho::expect_args name -- "$@"

  local lock_path
  if [[ "${name}" == */* ]]; then
    lock_path="${name}"
  else
    lock_path="${DYBATPHO_LOCK_DIR%/}/dybatpho-${name}"
  fi
  dybatpho::string_ends_with "${lock_path}" ".lock" || lock_path="${lock_path}.lock"
  printf '%s\n' "${lock_path}"
}

#######################################
# @description Read a single metadata field recorded for a lock.
# @arg $1 string Lock directory path
# @arg $2 string Field name (pid|host|command|acquired_at)
# @stdout Recorded value, or empty when the lock or field doesn't exist
#######################################
function dybatpho::lock_field {
  local lock_path field
  dybatpho::expect_args lock_path field -- "$@"
  local field_file="${lock_path}/${field}"
  if dybatpho::is file "${field_file}"; then
    cat "${field_file}"
  fi
}

#######################################
# @description Return success when the process that owns a lock is still alive on this host.
# @arg $1 string Lock directory path
# @exitcode 0 The recorded pid belongs to a live process on the current host
# @exitcode 1 The lock is missing, foreign to this host, or its process is gone (stale)
#######################################
function dybatpho::lock_is_alive {
  local lock_path
  dybatpho::expect_args lock_path -- "$@"
  dybatpho::is dir "${lock_path}" || return 1

  local pid host
  pid="$(dybatpho::lock_field "${lock_path}" pid)"
  host="$(dybatpho::lock_field "${lock_path}" host)"

  [[ -n "${pid}" ]] || return 1
  # A lock recorded on a different host can't be checked for liveness locally,
  # so conservatively treat it as still held.
  if [[ -n "${host}" && "${host}" != "$(dybatpho::lock_hostname)" ]]; then
    return 0
  fi
  kill -0 "${pid}" > /dev/null 2>&1
}

#######################################
# @description Return success when a lock is currently held by a live process.
# @arg $1 string Lock name or path
# @exitcode 0 The lock exists and is held by a live process
# @exitcode 1 The lock doesn't exist or is stale
#######################################
function dybatpho::lock_is_held {
  local name lock_path
  dybatpho::expect_args name -- "$@"
  lock_path="$(dybatpho::lock_path "${name}")"
  dybatpho::lock_is_alive "${lock_path}"
}

#######################################
# @description Print information about the process currently holding a lock.
# @arg $1 string Lock name or path
# @stdout `pid=<pid> host=<host> acquired_at=<timestamp> command=<command>` when held
# @exitcode 0 The lock is currently held and its info was printed
# @exitcode 1 The lock isn't held by anyone
#######################################
function dybatpho::lock_info {
  local name lock_path
  dybatpho::expect_args name -- "$@"
  lock_path="$(dybatpho::lock_path "${name}")"

  dybatpho::lock_is_alive "${lock_path}" || return 1
  printf 'pid=%s host=%s acquired_at=%s command=%s\n' \
    "$(dybatpho::lock_field "${lock_path}" pid)" \
    "$(dybatpho::lock_field "${lock_path}" host)" \
    "$(dybatpho::lock_field "${lock_path}" acquired_at)" \
    "$(dybatpho::lock_field "${lock_path}" command)"
}

#######################################
# @description Remove a lock directory left behind by a process that is no longer running.
# @arg $1 string Lock directory path
# @stderr Notice when a stale lock is reclaimed
#######################################
function dybatpho::lock_reclaim_stale {
  local lock_path
  dybatpho::expect_args lock_path -- "$@"
  if dybatpho::is dir "${lock_path}" && ! dybatpho::lock_is_alive "${lock_path}"; then
    dybatpho::warn "Reclaiming stale lock ${lock_path} (pid $(dybatpho::lock_field "${lock_path}" pid) is no longer running)"
    rm -rf "${lock_path}" > /dev/null 2>&1 || true
  fi
}

#######################################
# @description Acquire a portable, cross-platform (Linux/macOS) file lock, waiting up to a timeout.
# @example
#   dybatpho::lock_acquire "$(basename "$0")" || dybatpho::die "Already running"
#
# @example
#   dybatpho::lock_acquire "deploy" 30
#
# @arg $1 string Lock name (bare word resolved under `DYBATPHO_LOCK_DIR`) or an explicit path
# @arg $2 number Seconds to wait for the lock before giving up, default 0 (try once, don't wait)
# @env DYBATPHO_LOCK_DIR string Base directory used to resolve bare lock names
# @env DYBATPHO_LOCK_POLL_INTERVAL number Seconds to sleep between acquire attempts while waiting
# @stderr Info about the current holder when the lock can't be acquired
# @exitcode 0 The lock was acquired by the current process
# @exitcode 1 The lock is still held by another live process after the timeout elapses
# @tip Uses `mkdir` for atomic lock creation, so no dependency on `flock` is required
# @tip Pair with `dybatpho::lock_release` in a trap so the lock is always freed on exit
#######################################
function dybatpho::lock_acquire {
  local name timeout
  dybatpho::expect_args name -- "$@"
  timeout="${2:-0}"

  local lock_path
  lock_path="$(dybatpho::lock_path "${name}")"

  local start_time elapsed
  start_time="$(date +%s)"
  while true; do
    dybatpho::lock_reclaim_stale "${lock_path}"

    if mkdir "${lock_path}" 2> /dev/null; then
      printf '%s' "$$" > "${lock_path}/pid"
      printf '%s' "$(dybatpho::lock_hostname)" > "${lock_path}/host"
      printf '%s' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${lock_path}/acquired_at"
      printf '%s' "${DYBATPHO_LOCK_COMMAND:-${0}}" > "${lock_path}/command"
      return 0
    fi

    elapsed=$(($(date +%s) - start_time))
    if ((elapsed >= timeout)); then
      dybatpho::error "Could not acquire lock ${lock_path}: $(dybatpho::lock_info "${name}" 2> /dev/null || echo 'held by an unknown process')"
      return 1
    fi
    sleep "${DYBATPHO_LOCK_POLL_INTERVAL}"
  done
}

#######################################
# @description Release a lock previously acquired by the current process.
# @arg $1 string Lock name or path
# @exitcode 0 The lock was released, or wasn't held by the current process to begin with
# @exitcode 1 The lock is held by a different, still-live process and was left untouched
# @tip Safe to call even when the lock was never acquired by this process
#######################################
function dybatpho::lock_release {
  local name lock_path
  dybatpho::expect_args name -- "$@"
  lock_path="$(dybatpho::lock_path "${name}")"

  dybatpho::is dir "${lock_path}" || return 0

  local owner_pid
  owner_pid="$(dybatpho::lock_field "${lock_path}" pid)"
  if [[ -n "${owner_pid}" && "${owner_pid}" != "$$" ]] && dybatpho::lock_is_alive "${lock_path}"; then
    dybatpho::warn "Lock ${lock_path} is held by pid ${owner_pid}, not the current process; skipping release"
    return 1
  fi

  rm -rf "${lock_path}" > /dev/null 2>&1 || true
}

#######################################
# @description Acquire a lock, run a command while holding it, then release it, even if the command fails.
# @example
#   dybatpho::with_lock "deploy" 30 -- ./deploy.sh --env prod
#
# @arg $1 string Lock name or path
# @arg $2 number Seconds to wait for the lock before giving up
# @arg $3 string Literal `--` separating lock options from the command
# @arg $@ string Command and arguments to run while holding the lock
# @exitcode 1 The lock couldn't be acquired within the timeout
# @exitcode other Exit code of the wrapped command
#######################################
function dybatpho::with_lock {
  local name timeout separator
  dybatpho::expect_args name timeout separator -- "$@"
  shift 3
  [[ "${separator}" == "--" ]] || dybatpho::die "${FUNCNAME[0]}: Expected: name timeout -- command [args...]"
  (($# > 0)) || dybatpho::die "${FUNCNAME[0]}: Expected a command to run after --"

  dybatpho::lock_acquire "${name}" "${timeout}" || return 1

  local exit_code=0
  "$@" || exit_code=$?
  dybatpho::lock_release "${name}"
  return "${exit_code}"
}
