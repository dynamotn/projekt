setup() {
  load test_helper
  export DYBATPHO_LOCK_DIR="${BATS_TEST_TMPDIR}"
  export DYBATPHO_LOCK_POLL_INTERVAL="0.1"
}

teardown() {
  rm -rf "${DYBATPHO_LOCK_DIR}"/dybatpho-*.lock 2> /dev/null || true
}

@test "dybatpho::lock_path resolves bare name under DYBATPHO_LOCK_DIR" {
  assert_equal "$(dybatpho::lock_path "myjob")" "${DYBATPHO_LOCK_DIR}/dybatpho-myjob.lock"
}

@test "dybatpho::lock_path keeps an explicit path as-is" {
  assert_equal "$(dybatpho::lock_path "/tmp/custom/myjob")" "/tmp/custom/myjob.lock"
}

@test "dybatpho::lock_path doesn't double the .lock suffix" {
  assert_equal "$(dybatpho::lock_path "myjob.lock")" "${DYBATPHO_LOCK_DIR}/dybatpho-myjob.lock"
}

@test "dybatpho::lock_acquire then dybatpho::lock_is_held reports success" {
  run dybatpho::lock_acquire "acquire-test"
  assert_success

  run dybatpho::lock_is_held "acquire-test"
  assert_success

  dybatpho::lock_release "acquire-test"
}

@test "dybatpho::lock_acquire fails fast without waiting when already held" {
  dybatpho::lock_acquire "busy"

  run --separate-stderr dybatpho::lock_acquire "busy"
  assert_failure
  assert_stderr --partial "Could not acquire lock"
  assert_stderr --partial "pid=$$"

  dybatpho::lock_release "busy"
}

@test "dybatpho::lock_acquire waits up to the timeout then succeeds after release" {
  dybatpho::lock_acquire "waiter"
  (
    sleep 0.3
    dybatpho::lock_release "waiter"
  ) &
  local releaser_pid=$!

  run dybatpho::lock_acquire "waiter" 2
  assert_success
  wait "${releaser_pid}"
  dybatpho::lock_release "waiter"
}

@test "dybatpho::lock_acquire waits up to the timeout then fails when still held" {
  dybatpho::lock_acquire "still-busy"
  run --separate-stderr dybatpho::lock_acquire "still-busy" 1
  assert_failure
  dybatpho::lock_release "still-busy"
}

@test "dybatpho::lock_info prints the current holder metadata" {
  dybatpho::lock_acquire "info-test"
  run dybatpho::lock_info "info-test"
  assert_success
  assert_output --partial "pid=$$"
  assert_output --partial "host=$(dybatpho::lock_hostname)"
  assert_output --partial "acquired_at="
  dybatpho::lock_release "info-test"
}

@test "dybatpho::lock_info fails when the lock isn't held" {
  run dybatpho::lock_info "never-acquired"
  assert_failure
  refute_output
}

@test "dybatpho::lock_release removes a lock held by the current process" {
  dybatpho::lock_acquire "release-test"
  run dybatpho::lock_release "release-test"
  assert_success
  run dybatpho::lock_is_held "release-test"
  assert_failure
}

@test "dybatpho::lock_release is a no-op when the lock was never held" {
  run dybatpho::lock_release "never-held"
  assert_success
}

@test "dybatpho::lock_release refuses to remove a lock held by another live process" {
  sleep 5 &
  local foreign_pid=$!

  local lock_path
  lock_path="$(dybatpho::lock_path "foreign")"
  mkdir "${lock_path}"
  printf '%s' "${foreign_pid}" > "${lock_path}/pid"
  printf '%s' "$(dybatpho::lock_hostname)" > "${lock_path}/host"

  run --separate-stderr dybatpho::lock_release "foreign"
  assert_failure
  assert_stderr --partial "is held by pid ${foreign_pid}"
  assert_dir_exist "${lock_path}"

  kill "${foreign_pid}" 2> /dev/null || true
  rm -rf "${lock_path}"
}

@test "dybatpho::lock_acquire reclaims a stale lock left by a dead process" {
  local lock_path
  lock_path="$(dybatpho::lock_path "stale")"
  mkdir "${lock_path}"
  # 999999 is exceedingly unlikely to be a running pid in the test environment.
  printf '%s' "999999" > "${lock_path}/pid"
  printf '%s' "$(dybatpho::lock_hostname)" > "${lock_path}/host"

  run --separate-stderr dybatpho::lock_acquire "stale"
  assert_success
  assert_stderr --partial "Reclaiming stale lock"
  dybatpho::lock_release "stale"
}

@test "dybatpho::with_lock runs the command while holding the lock and releases it after" {
  run dybatpho::with_lock "with-lock-test" 1 -- bash -c 'echo ran'
  assert_success
  assert_output "ran"
  run dybatpho::lock_is_held "with-lock-test"
  assert_failure
}

@test "dybatpho::with_lock releases the lock even when the command fails" {
  run dybatpho::with_lock "with-lock-fail" 1 -- bash -c 'exit 5'
  assert_failure 5
  run dybatpho::lock_is_held "with-lock-fail"
  assert_failure
}

@test "dybatpho::with_lock requires a -- separator" {
  run dybatpho::with_lock "with-lock-sep" 1 bash -c 'echo ran'
  assert_failure
}
