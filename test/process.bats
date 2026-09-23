setup() {
  load test_helper
}

@test 'dybatpho::die output' {
  local exit_code=7
  run --separate-stderr -"${exit_code}" dybatpho::die loioday "${exit_code}"
  assert_failure
  refute_output
  assert_stderr --partial "loioday"
}

@test 'dybatpho::register_err_handler output' {
  run --separate-stderr dybatpho::register_err_handler
  assert_success
  refute_output
  refute_stderr
}

@test 'dybatpho::register_killed_handler output' {
  run --separate-stderr dybatpho::register_killed_handler
  assert_success
  refute_output
  refute_stderr
}

@test 'dybatpho::register_common_handlers output' {
  run --separate-stderr dybatpho::register_common_handlers
  assert_success
  refute_output
  refute_stderr
}

@test 'dybatpho::register_common_handlers installs traps in the current shell' {
  dybatpho::register_common_handlers
  local err_trap sigint_trap sigterm_trap
  err_trap="$(trap -p ERR)"
  sigint_trap="$(trap -p SIGINT)"
  sigterm_trap="$(trap -p SIGTERM)"
  # Restore the default traps before asserting so a failure can't loop back in.
  trap - ERR SIGINT SIGTERM
  set +E

  assert_equal "${DYBATPHO_USED_ERR_HANDLER}" "true"
  assert_equal "${DYBATPHO_USED_KILLED_HANDLER}" "true"
  assert_regex "${err_trap}" "run_err_handler"
  # A signal inherited as ignored (bats --jobs does this) can't be trapped again.
  if [[ "${sigint_trap}" != *"-- ''"* ]]; then
    assert_regex "${sigint_trap}" "killed_process_handler SIGINT"
    assert_regex "${sigterm_trap}" "killed_process_handler SIGTERM"
  fi
}

@test 'dybatpho::run_err_handler output' {
  local exit_code=7
  run --separate-stderr -"${exit_code}" dybatpho::run_err_handler "${exit_code}"
  assert_failure
  refute_output
  assert_stderr
}

@test 'dybatpho::killed_process_handler output' {
  run --separate-stderr dybatpho::killed_process_handler SIGTERM
  assert_failure
  refute_output
  assert_stderr

  run --separate-stderr dybatpho::killed_process_handler SIGINT
  assert_failure
  refute_output
  assert_stderr
}

@test 'dybatpho::trap on exit' {
  run --separate-stderr dybatpho::trap 'echo 2; echo 3' ERR EXIT
  assert_success
  assert_line --index 0 2
  assert_line --index 1 3
  refute_stderr
}

@test 'dybatpho::trap preserves existing trap handlers' {
  # shellcheck disable=2329
  _trap_chain() {
    trap 'echo first' EXIT
    dybatpho::trap 'echo second' EXIT
  }
  # The chained EXIT trap must fire in a subshell, so keep the isolating `run`.
  run _trap_chain
  assert_success
  assert_line --index 0 first
  assert_line --index 1 second
}

@test 'dybatpho::cleanup_file_on_exit action' {
  # Just test that function runs without error and registers trap
  local filepath="$(mktemp -p "${BATS_TEST_TMPDIR}")"
  run --separate-stderr dybatpho::cleanup_file_on_exit "${filepath}"
  assert_success
  refute_output
  refute_stderr

  # Cleanup is triggered on EXIT, file may or may not be deleted immediately
  # Verify cleanup script was created
  local cleanup_scripts=$(ls /tmp/dybatpho_cleanup-*.sh 2> /dev/null | wc -l)
  [[ "${cleanup_scripts}" -gt 0 ]]
}

@test 'dybatpho::cleanup_file_on_exit removes file on shell exit' {
  local filepath="${BATS_TEST_TMPDIR}/cleanup-me"
  # shellcheck disable=2329
  _register_cleanup() {
    local file="$1"
    touch "${file}"
    dybatpho::cleanup_file_on_exit "${file}"
    echo "${file}"
  }
  run _register_cleanup "${filepath}"
  assert_success
  assert_output "${filepath}"
  assert_file_not_exist "${filepath}"
}

@test 'dybatpho::cleanup_file_on_exit leaves the Bats reporter on EXIT alone' {
  # Bats reports a test result from its own EXIT trap. Replacing that trap in
  # the test shell hid every failure in every test that made a temporary file:
  # the test vanished from the report and the run ended with "Executed N-1
  # instead of N tests", which reads as a dead worker rather than a failure.
  local before after
  before="$(trap -p EXIT)"
  assert_regex "${before}" 'bats_teardown_trap'

  dybatpho::cleanup_file_on_exit "${BATS_TEST_TMPDIR}/never-created"

  after="$(trap -p EXIT)"
  assert_equal "${after}" "${before}"
}

@test 'dybatpho::cleanup_file_on_exit still cleans up when a subshell ends' {
  # The other half of the rule: a subshell has no reporter to protect, and its
  # exit is the only chance to remove what it registered.
  local marker="${BATS_TEST_TMPDIR}/subshell-path"
  (
    local scratch
    dybatpho::create_temp scratch ".txt" "subshell-cleanup"
    printf '%s' "${scratch}" > "${marker}"
    [[ -f "${scratch}" ]]
  )
  assert_file_not_exist "$(cat "${marker}")"
}

@test 'dybatpho::cleanup_file_on_exit removes directory on shell exit' {
  local dirpath="${BATS_TEST_TMPDIR}/cleanup-dir"
  _register_cleanup_dir() {
    local dir="$1"
    mkdir -p "${dir}"
    dybatpho::cleanup_file_on_exit "${dir}"
    echo "${dir}"
  }
  assert_equal "$(_register_cleanup_dir "${dirpath}")" "${dirpath}"
  [[ ! -e "${dirpath}" ]]
}

@test 'dybatpho::cleanup_file_on_exit removes a path containing spaces' {
  local target="${BATS_TEST_TMPDIR}/dir with space"
  mkdir -p "${target}"
  # A `bash -c` shell has an empty `BASH_SOURCE`, which the kcov hook expands on
  # every command and `set -u` then turns into a failure that shows up only
  # under `scripts/test.sh --coverage`. Spawn from a script file instead.
  local script="${BATS_TEST_TMPDIR}/cleanup_spaces.sh"
  cat > "${script}" << 'SCRIPT'
. "${1}/init.sh"
dybatpho::cleanup_file_on_exit "${2}"
SCRIPT
  bash "${script}" "${DYBATPHO_DIR}" "${target}"
  assert_dir_not_exist "${target}"
}

@test 'dybatpho::cleanup_file_on_exit installs one trap for many paths' {
  local first="${BATS_TEST_TMPDIR}/first" second="${BATS_TEST_TMPDIR}/second"
  touch "${first}" "${second}"
  # Every registration used to append its own command, so the trap grew with
  # each temporary file. It is now a single call into the path registry.
  local script="${BATS_TEST_TMPDIR}/cleanup_one_trap.sh"
  cat > "${script}" << 'SCRIPT'
. "${1}/init.sh"
dybatpho::cleanup_file_on_exit "${2}"
dybatpho::cleanup_file_on_exit "${3}"
trap -p EXIT
SCRIPT
  run bash "${script}" "${DYBATPHO_DIR}" "${first}" "${second}"
  assert_success
  assert_equal "$(printf '%s\n' "${output}" | grep -c '__dybatpho_cleanup_run')" "1"
}

@test 'dybatpho::cleanup_file_on_exit leaves paths registered by another shell alone' {
  local outer="${BATS_TEST_TMPDIR}/outer"
  touch "${outer}"
  # The subshell inherits the registry, so it must remove only what it
  # registered itself; the parent still needs `outer`.
  local script="${BATS_TEST_TMPDIR}/cleanup_other_shell.sh"
  cat > "${script}" << 'SCRIPT'
. "${1}/init.sh"
dybatpho::cleanup_file_on_exit "${2}"
(:)
[[ -e "${2}" ]] || {
  echo 'removed too early'
  exit 1
}
SCRIPT
  run bash "${script}" "${DYBATPHO_DIR}" "${outer}"
  assert_success
  assert_file_not_exist "${outer}"
}

@test "dybatpho::dry_run with DRY_RUN=true should print dry run message and not execute command" {
  # shellcheck disable=2030
  export DRY_RUN="true"
  local test_file="dry_run_test_file.tmp"
  rm -f "${test_file}"
  run dybatpho::dry_run touch "${test_file}"
  assert_output --partial "DRY RUN: touch ${test_file}"
  assert_file_not_exist "${test_file}"
  unset DRY_RUN
}

@test "dybatpho::dry_run with DRY_RUN=true displays args with spaces as single quoted tokens" {
  # shellcheck disable=2030
  export DRY_RUN="true"
  run_traced dybatpho::dry_run curl -H "Authorization: Bearer token"
  assert_output --partial "DRY RUN:"
  # The header value must be shell-quoted as a single token (printf %q uses backslash escaping)
  assert_output --partial "Authorization:"
  assert_output --partial "Bearer"
  # All parts must appear on the same line, not split across lines
  local line
  while IFS= read -r line; do
    if [[ "${line}" == *"Authorization:"* ]]; then
      [[ "${line}" == *"Bearer"* ]] || {
        echo "header value was split across lines"
        return 1
      }
    fi
  done <<< "${output}"
  unset DRY_RUN
}

@test "dybatpho::dry_run with DRY_RUN=false preserves argument quoting during execution" {
  # shellcheck disable=2031
  export DRY_RUN="false"
  # Use printf to write args one-per-line; verify "hello world" stays as one arg
  local temp_file="${BATS_TEST_TMPDIR}/dry_run_exec"
  assert_equal "$(dybatpho::dry_run bash -c "printf '%s\n' \"\$@\"" -- "hello world" "second")" "$(printf 'hello world\nsecond')"

  # A single argument is evaluated as a shell command string.
  assert_equal "$(dybatpho::dry_run "printf 'evaluated\n'")" "evaluated"
  unset DRY_RUN
}

@test "dybatpho::dry_run with DRY_RUN=false should execute command and produce no dry run output" {
  # shellcheck disable=2031
  export DRY_RUN="false"
  local test_file="actual_run_test_file.tmp"
  rm -f "${test_file}"
  run dybatpho::dry_run touch "${test_file}"
  assert_output ""
  refute_output --regexp "DRY RUN:"
  assert_file_exist "${test_file}"
  rm -f "${test_file}"
  unset DRY_RUN
}

@test "dybatpho::dry_run with DRY_RUN unset should error" {
  unset DRY_RUN
  local test_file="unset_dry_run_test_file.tmp"
  rm -f "${test_file}"
  run dybatpho::dry_run touch "${test_file}"
  assert_failure
}
