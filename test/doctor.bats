setup() {
  load test_helper
  # The report reads PATH, so every test decides for itself what is installed.
  DOCTOR_BIN="${BATS_TEST_TMPDIR}/bin"
  mkdir -p "${DOCTOR_BIN}"
  ORIGINAL_PATH="${PATH}"
}

teardown() {
  PATH="${ORIGINAL_PATH}"
}

# Put a fake command on a PATH that holds nothing else, so "installed" means
# exactly what the test says it means.
fake_command() {
  printf '#!/usr/bin/env bash\nexit 0\n' > "${DOCTOR_BIN}/$1"
  # `only_fakes` may already have narrowed PATH down to the fake directory,
  # which is exactly the directory `chmod` is not in.
  PATH="${ORIGINAL_PATH}" chmod +x "${DOCTOR_BIN}/$1"
}

only_fakes() {
  PATH="${DOCTOR_BIN}"
}

@test "dybatpho::doctor_requirements lists required dependencies" {
  run -0 dybatpho::doctor_requirements archive required
  assert_output "tar"
}

@test "dybatpho::doctor_requirements lists optional dependencies" {
  run -0 dybatpho::doctor_requirements json optional
  assert_output "jq"
}

@test "dybatpho::doctor_requirements lists the host probes of the os module" {
  run -0 dybatpho::doctor_requirements os optional
  assert_line --index 0 "hostname"
  assert_line --index 1 "nproc|sysctl|getconf"
  assert_line --index 2 "tput"
}

@test "dybatpho::doctor_requirements defaults to every kind" {
  run -0 dybatpho::doctor_requirements json
  assert_line --index 0 "yq"
  assert_line --index 1 "jq"
}

@test "dybatpho::doctor_requirements prints nothing for a module without dependencies" {
  run -0 dybatpho::doctor_requirements string
  assert_output ""
}

@test "dybatpho::doctor_requirements rejects an unknown module" {
  run -1 dybatpho::doctor_requirements nosuch
  assert_output --partial "Unknown module 'nosuch'"
}

@test "dybatpho::doctor_requirements rejects an unknown kind" {
  run -1 dybatpho::doctor_requirements json sometimes
  assert_output --partial "Unknown kind 'sometimes'"
}

@test "dybatpho::doctor_bash_supported accepts the running shell" {
  run -0 dybatpho::doctor_bash_supported
}

@test "dybatpho::doctor reports the version, the bash version and the modules" {
  run -0 dybatpho::doctor --modules string
  assert_line --index 0 --partial "dybatpho $(dybatpho::version)"
  assert_output --partial "minimum 4.3"
  assert_output --partial "modules  string"
}

@test "dybatpho::doctor says so when a module set needs nothing external" {
  run -0 dybatpho::doctor --modules "string array"
  assert_output --partial "No external dependency is needed"
}

@test "dybatpho::doctor marks an installed dependency ok with its path" {
  only_fakes
  fake_command curl
  run -0 dybatpho::doctor --modules network
  assert_output --partial "curl"
  assert_output --partial "ok (${DOCTOR_BIN}/curl)"
}

@test "dybatpho::doctor fails when a required dependency is missing" {
  only_fakes
  run -1 dybatpho::doctor --modules network
  assert_output --partial "Missing required: curl"
}

@test "dybatpho::doctor succeeds when only optional dependencies are missing" {
  only_fakes
  fake_command curl
  run -0 dybatpho::doctor --modules network
  assert_output --partial "Optional, some functions are unavailable:"
  assert_output --partial "No required dependency is missing."
}

@test "dybatpho::doctor accepts any alternative of an any-of dependency" {
  only_fakes
  fake_command shasum
  run -0 dybatpho::doctor --modules file
  assert_output --partial "sha256sum|shasum|openssl"
  assert_output --partial "ok (${DOCTOR_BIN}/shasum)"
}

@test "dybatpho::doctor reports an any-of dependency missing only when every alternative is" {
  only_fakes
  run -0 dybatpho::doctor --modules file
  assert_output --partial "sha256sum|shasum|openssl"
  assert_output --partial "missing"
}

@test "dybatpho::doctor accepts a comma separated module list" {
  run -0 dybatpho::doctor --modules "string,array"
  assert_output --partial "modules  string array"
}

@test "dybatpho::doctor covers the registry with --all" {
  run dybatpho::doctor --all
  assert_output --partial "archive"
  assert_output --partial "git"
}

@test "dybatpho::doctor defaults to the loaded modules" {
  run dybatpho::doctor
  assert_output --partial "modules  ${DYBATPHO_LOADED_MODULES}"
}

@test "dybatpho::doctor --quiet prints nothing and reports through the exit code" {
  only_fakes
  run -1 dybatpho::doctor --modules network --quiet
  assert_output ""
  fake_command curl
  run -0 dybatpho::doctor --modules network --quiet
  assert_output ""
}

@test "dybatpho::doctor --json prints one object with every dependency" {
  only_fakes
  fake_command curl
  fake_command yq
  run -0 dybatpho::doctor --modules "network,json" --json
  assert_output --partial '"version":"'
  assert_output --partial '"bash":{"version":'
  assert_output --partial '"modules":["network","json"]'
  assert_output --partial '{"module":"network","dependency":"curl","kind":"required","status":"ok"'
  assert_output --partial '"ok":true'
}

@test "dybatpho::doctor --json reports ok false when a required dependency is missing" {
  only_fakes
  run -1 dybatpho::doctor --modules network --json
  assert_output --partial '"status":"missing"'
  assert_output --partial '"ok":false'
}

@test "dybatpho::doctor --json output parses as JSON" {
  if ! dybatpho::is command jq; then
    skip "jq is not installed"
  fi
  run -0 dybatpho::doctor --modules "string array" --json
  printf '%s' "${output}" | jq -e '.bash.ok == true' > /dev/null
}

@test "dybatpho::doctor rejects an unknown module" {
  run -1 dybatpho::doctor --modules nosuch
  assert_output --partial "Unknown module 'nosuch'"
}

@test "dybatpho::doctor rejects an unknown option" {
  run -1 dybatpho::doctor --nope
  assert_output --partial "Unknown option '--nope'"
}

@test "dybatpho::doctor rejects --modules without a value" {
  run -1 dybatpho::doctor --modules
  assert_output --partial "--modules expects a module list"
}
