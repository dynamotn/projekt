setup() {
  load test_helper
}

@test "config_load loads dotenv files and later files override earlier values" {
  local base="${BATS_TEST_TMPDIR}/base.env"
  local local_config="${BATS_TEST_TMPDIR}/local.env"
  printf 'HOST=example.test\nPORT=80\n' > "${base}"
  printf 'PORT=443\nMESSAGE="hello world"\n' > "${local_config}"

  dybatpho::config_load "${base}" "${local_config}"
  assert_equal "$(dybatpho::config_get HOST)" "example.test"
  assert_equal "$(dybatpho::config_get PORT)" "443"
  assert_equal "$(dybatpho::config_get MESSAGE)" "hello world"
}

@test "config_env applies prefixed environment variables" {
  export APP_HOST="env.test"
  dybatpho::config_env APP_
  assert_equal "$(dybatpho::config_get HOST)" "env.test"
  unset APP_HOST
}

@test "config_get supports defaults and config_export exports values" {
  assert_equal "$(dybatpho::config_get MISSING fallback)" "fallback"

  __dybatpho_config_set EXPORTED "value"
  dybatpho::config_export
  assert_equal "${EXPORTED}" "value"
}

@test "config_load rejects unsupported files and missing files" {
  run dybatpho::config_load "${BATS_TEST_TMPDIR}/missing.conf"
  assert_failure
  assert_output --partial "Configuration file not found"

  local config="${BATS_TEST_TMPDIR}/config.txt"
  : > "${config}"
  run dybatpho::config_load "${config}"
  assert_failure
  assert_output --partial "Unsupported configuration format"
}

@test "config_load parses dotenv comments, quoting, and escaped values" {
  local config="${BATS_TEST_TMPDIR}/quoted.dotenv"
  printf '%s\n' \
    '# comment' \
    'PLAIN=value # inline comment' \
    'DOUBLE="line\nvalue"' \
    "SINGLE='literal # value'" \
    'SPACED = trimmed' > "${config}"

  DYBATPHO_CONFIG=()
  dybatpho::config_load "${config}"
  assert_equal "$(dybatpho::config_get PLAIN)" "value"
  assert_equal "$(dybatpho::config_get DOUBLE)" $'line\nvalue'
  assert_equal "$(dybatpho::config_get SINGLE)" 'literal # value'
  assert_equal "$(dybatpho::config_get SPACED)" "trimmed"
}

@test "config_load supports JSON and YAML files with precedence" {
  local json_file="${BATS_TEST_TMPDIR}/settings.json"
  local yaml_file="${BATS_TEST_TMPDIR}/settings.yaml"
  printf '{}' > "${json_file}"
  printf '{}' > "${yaml_file}"
  stub jq ": printf 'PORT\\t8080\\nSHARED\\tfrom-json\\n'"
  stub yq ": printf 'SHARED\\tfrom-yaml\\nHOST\\tlocalhost\\n'"

  DYBATPHO_CONFIG=()
  dybatpho::config_load "${json_file}" "${yaml_file}"
  assert_equal "$(dybatpho::config_get PORT)" "8080"
  assert_equal "$(dybatpho::config_get SHARED)" "from-yaml"
  assert_equal "$(dybatpho::config_get HOST)" "localhost"
  unstub jq
  unstub yq
}

@test "config_load reports invalid dotenv and structured configuration" {
  local dotenv="${BATS_TEST_TMPDIR}/invalid.env"
  local json_file="${BATS_TEST_TMPDIR}/invalid.json"
  local yaml_file="${BATS_TEST_TMPDIR}/invalid.yaml"
  printf 'not an assignment\n' > "${dotenv}"
  run dybatpho::config_load "${dotenv}"
  assert_failure
  assert_output --partial "Invalid dotenv entry"

  printf '{}' > "${json_file}"
  stub jq ": exit 1"
  run dybatpho::config_load "${json_file}"
  assert_failure
  assert_output --partial "Invalid JSON configuration"
  unstub jq

  printf '{}' > "${yaml_file}"
  stub yq ": exit 1"
  run dybatpho::config_load "${yaml_file}"
  assert_failure
  assert_output --partial "Invalid YAML configuration"
  unstub yq
}

@test "config_get and config_require validate keys and missing values" {
  DYBATPHO_CONFIG=()
  run ! dybatpho::config_get MISSING
  __dybatpho_config_set PRESENT "yes"
  dybatpho::config_require PRESENT

  run --separate-stderr dybatpho::config_get "bad key"
  assert_failure
  assert_stderr --partial "Invalid configuration key"

  run dybatpho::config_require
  assert_failure

  run --separate-stderr dybatpho::config_require MISSING
  assert_failure
  assert_stderr --partial "Required configuration is missing"
}

@test "config_env applies only matching prefixed variables" {
  export DYBATPHO_CONFIG_ENV_TEST="loaded"
  export UNRELATED_CONFIG_ENV_TEST="ignored"
  DYBATPHO_CONFIG=()
  dybatpho::config_env DYBATPHO_CONFIG_
  assert_equal "$(dybatpho::config_get ENV_TEST)" "loaded"
  run dybatpho::config_get UNRELATED_CONFIG_ENV_TEST
  assert_failure
  unset DYBATPHO_CONFIG_ENV_TEST UNRELATED_CONFIG_ENV_TEST
}

@test "config_env also loads variables without a prefix" {
  export DYBATPHO_CONFIG_UNPREFIXED="loaded"
  DYBATPHO_CONFIG=()
  set +u
  dybatpho::config_env
  set -u
  assert_equal "$(dybatpho::config_get DYBATPHO_CONFIG_UNPREFIXED)" "loaded"
  unset DYBATPHO_CONFIG_UNPREFIXED
}

@test "config_load rejects invalid keys returned by structured backends" {
  local json_file="${BATS_TEST_TMPDIR}/bad-key.json"
  printf '{}' > "${json_file}"
  stub jq ": printf '1BAD\\tvalue\\n'"
  run dybatpho::config_load "${json_file}"
  assert_failure
  assert_output --partial "Invalid configuration key"
  unstub jq
}

@test "config_export rejects invalid prefixes and non-shell keys" {
  DYBATPHO_CONFIG=()
  run --separate-stderr dybatpho::config_export "bad-prefix-"
  assert_failure
  assert_stderr --partial "Invalid configuration variable prefix"

  __dybatpho_config_set "with.dot" "value"
  run --separate-stderr dybatpho::config_export
  assert_failure
  assert_stderr --partial "Cannot export configuration key as variable"
}

@test "config_validate applies defaults and validates types, ranges, URLs, and enums" {
  DYBATPHO_CONFIG=()
  dybatpho::config_schema_reset
  dybatpho::config_schema HOST url required:true
  dybatpho::config_schema PORT int default:8080 min:1 max:65535
  dybatpho::config_schema MODE enum choices:dev,prod
  dybatpho::config_schema DEBUG bool
  __dybatpho_config_set HOST "https://example.test"
  __dybatpho_config_set MODE prod
  __dybatpho_config_set DEBUG true
  # An optional key without a default is skipped instead of failing.
  dybatpho::config_schema REGION string
  dybatpho::config_validate
  assert_equal "$(dybatpho::config_get PORT)" "8080"
  run ! dybatpho::config_get REGION
}

@test "config_validate rejects missing required and invalid values" {
  DYBATPHO_CONFIG=()
  dybatpho::config_schema_reset
  dybatpho::config_schema REQUIRED string required:true
  run --separate-stderr dybatpho::config_validate
  assert_failure
  assert_stderr --partial "required value is missing"

  DYBATPHO_CONFIG=()
  dybatpho::config_schema_reset
  dybatpho::config_schema PORT int min:1 max:10
  __dybatpho_config_set PORT 99
  run --separate-stderr dybatpho::config_validate
  assert_failure
  assert_stderr --partial "must be at most 10"

  DYBATPHO_CONFIG=()
  dybatpho::config_schema_reset
  dybatpho::config_schema MODE enum choices:dev,prod
  __dybatpho_config_set MODE test
  run --separate-stderr dybatpho::config_validate
  assert_failure
  assert_stderr --partial "expected one of"
}

@test "config_schema rejects invalid types and rules" {
  run --separate-stderr dybatpho::config_schema VALUE float
  assert_failure
  assert_stderr --partial "Unsupported configuration type"
  run --separate-stderr dybatpho::config_schema VALUE string unknown:value
  assert_failure
  assert_stderr --partial "Unsupported configuration schema rule"
}

@test "config_schema accepts long type aliases and replaces earlier declarations" {
  DYBATPHO_CONFIG=()
  dybatpho::config_schema_reset
  dybatpho::config_schema PORT integer min:1 max:65535
  dybatpho::config_schema DEBUG boolean default:no
  # Re-declaring a key replaces its rules instead of merging them.
  dybatpho::config_schema PORT integer default:9000
  __dybatpho_config_set PORT 70000
  dybatpho::config_validate
  assert_equal "$(dybatpho::config_get DEBUG)" "no"
  assert_equal "${#DYBATPHO_CONFIG_SCHEMA_KEYS[@]}" "2"
}

@test "config_validate applies string length ranges and reports every failing key" {
  DYBATPHO_CONFIG=()
  dybatpho::config_schema_reset
  dybatpho::config_schema NAME string min:3 max:5
  dybatpho::config_schema PORT int
  dybatpho::config_schema ENDPOINT url required:true
  __dybatpho_config_set NAME "ab"
  __dybatpho_config_set PORT "eight"

  run --separate-stderr dybatpho::config_validate
  assert_failure
  assert_stderr --partial "\`NAME\`: must be at least 3 characters"
  assert_stderr --partial "\`PORT\`: expected an integer"
  assert_stderr --partial "\`ENDPOINT\`: required value is missing"
}

@test "config_validate prefers a declared default over a required failure" {
  DYBATPHO_CONFIG=()
  dybatpho::config_schema_reset
  dybatpho::config_schema REGION string required:true default:us-east-1
  dybatpho::config_validate
  assert_equal "$(dybatpho::config_get REGION)" "us-east-1"
  assert_equal "${#DYBATPHO_CONFIG_ERRORS[@]}" "0"
}

@test "config_validate accepts every supported boolean and rejects other values" {
  DYBATPHO_CONFIG=()
  dybatpho::config_schema_reset
  dybatpho::config_schema FLAG bool
  local value
  for value in true False YES no ON off 1 0; do
    __dybatpho_config_set FLAG "${value}"
    dybatpho::config_validate
  done

  __dybatpho_config_set FLAG maybe
  run --separate-stderr dybatpho::config_validate
  assert_failure
  assert_stderr --partial "expected a boolean"
}

@test "config_schema rejects malformed rule values and enums without choices" {
  dybatpho::config_schema_reset
  run --separate-stderr dybatpho::config_schema PORT int min:many
  assert_failure
  assert_stderr --partial "Invalid \`min\` rule for PORT"

  run --separate-stderr dybatpho::config_schema FLAG bool required:sometimes
  assert_failure
  assert_stderr --partial "Invalid \`required\` rule for FLAG"

  run --separate-stderr dybatpho::config_schema MODE enum
  assert_failure
  assert_stderr --partial "requires \`choices\`"

  run --separate-stderr dybatpho::config_schema MODE enum choices:
  assert_failure
  assert_stderr --partial "Empty \`choices\` rule for MODE"

  run --separate-stderr dybatpho::config_schema "bad key" string
  assert_failure
  assert_stderr --partial "Invalid configuration key"

  run --separate-stderr dybatpho::config_schema VALUE string nocolon
  assert_failure
  assert_stderr --partial "Invalid configuration schema rule"
}

@test "config_doc renders markdown, text, and JSON references in declaration order" {
  dybatpho::config_schema_reset
  dybatpho::config_schema HOST url required:true description:"API base URL"
  dybatpho::config_schema PORT int default:8080 min:1 max:65535
  dybatpho::config_schema MODE enum choices:dev,prod default:dev

  run dybatpho::config_doc
  assert_success
  assert_line --index 0 "# Configuration"
  assert_line --partial "| \`HOST\` | url | true | - | - | API base URL |"
  assert_line --partial "| \`PORT\` | int | false | \`8080\` | 1..65535 | - |"
  assert_line --partial "| \`MODE\` | enum | false | \`dev\` | one of: dev, prod | - |"

  run dybatpho::config_doc text "App settings"
  assert_success
  assert_line --index 0 "App settings"
  assert_line --partial "  type: url"
  assert_line --partial "  constraints: 1..65535"
  assert_line --partial "  description: API base URL"

  run dybatpho::config_doc json
  assert_success
  assert_output --partial '{"key":"HOST","type":"url","required":true,"default":null,"constraints":null,"description":"API base URL"}'
  assert_output --partial '{"key":"PORT","type":"int","required":false,"default":"8080","constraints":"1..65535","description":null}'
}

@test "config_doc reports unsupported formats and renders an empty schema" {
  dybatpho::config_schema_reset
  run --separate-stderr dybatpho::config_doc xml
  assert_failure
  assert_stderr --partial "Unsupported configuration documentation format"

  run dybatpho::config_doc json
  assert_success
  assert_output "[]"
}
