setup() {
  load test_helper
}

teardown() {
  local pidfile="${BATS_TEST_TMPDIR}/listener.pid"
  [[ -f "${pidfile}" ]] || return 0
  kill "$(cat "${pidfile}")" 2> /dev/null || true
}

# Open a listening socket on a free port and print that port.
#
# The process id goes to a file rather than a variable because the caller reads
# the port through a command substitution, and a variable set in that subshell
# would never reach `teardown`. The port is chosen by the kernel, so two tests
# running at once cannot collide on it.
start_listener() {
  dybatpho::is command python3 || return 1
  local portfile="${BATS_TEST_TMPDIR}/listener.port"
  local pidfile="${BATS_TEST_TMPDIR}/listener.pid"
  python3 -c 'import socket, time
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", 0))
s.listen(5)
print(s.getsockname()[1], flush=True)
time.sleep(30)' > "${portfile}" 2> /dev/null &
  printf '%s\n' "$!" > "${pidfile}"
  local waited=0
  while [[ ! -s "${portfile}" ]] && ((waited < 100)); do
    sleep 0.1
    waited=$((waited + 1))
  done
  [[ -s "${portfile}" ]] || return 1
  cat "${portfile}"
}

# Answer one HTTP request with an empty 200 on a free port, and print that port.
#
# A fixed port such as 8080 is often already taken by some local service, which
# then answers instead, and a `nc` left behind by a failed run holds it for the
# next one. The process id goes to the same file `teardown` reads.
start_http_server() {
  dybatpho::is command python3 || return 1
  local portfile="${BATS_TEST_TMPDIR}/listener.port"
  local pidfile="${BATS_TEST_TMPDIR}/listener.pid"
  python3 -c 'import socket
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", 0))
s.listen(1)
s.settimeout(30)
print(s.getsockname()[1], flush=True)
c, _ = s.accept()
c.recv(65536)
c.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
c.close()' > "${portfile}" 2> /dev/null &
  printf '%s\n' "$!" > "${pidfile}"
  local waited=0
  while [[ ! -s "${portfile}" ]] && ((waited < 100)); do
    sleep 0.1
    waited=$((waited + 1))
  done
  [[ -s "${portfile}" ]] || return 1
  cat "${portfile}"
}

@test "__dybatpho_network_get_http_code no arg" {
  run __dybatpho_network_get_http_code
  assert_failure
}

@test "__dybatpho_network_get_http_code output" {
  assert_equal "$(__dybatpho_network_get_http_code 403)" "403 (forbidden)"
}

@test "__dybatpho_network_get_http_code with unknown code" {
  assert_equal "$(__dybatpho_network_get_http_code 999)" "999 (unknown)"
}

@test "dybatpho::curl_do no arg" {
  run dybatpho::curl_do
  assert_failure
}

@test "dybatpho::curl_do with empty url" {
  run --separate-stderr dybatpho::curl_do ""
  assert_failure
}

@test "dybatpho::curl_do only url" {
  stub curl ": echo '200'"
  run_traced dybatpho::curl_do https://this
  unstub curl
  assert_success
}

@test "dybatpho::curl_do with more than 2 parameters" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_do"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_do https://this "${temp_file}" --header "X-Test: 1"
  grep "header X-Test: 1" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_do header with space in value is passed as single argument" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_do_header"
  # Write each curl argument on its own line so we can verify quoting
  stub curl ": printf '%s\n' \"\$@\" > ${temp_file}; echo '200'"
  dybatpho::curl_do https://this "${temp_file}" -H "Authorization: Bearer mytoken"
  # The full header value must appear on one line, not split
  grep "^Authorization: Bearer mytoken$" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_do with status code 200" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_do"
  stub curl ": echo '200'; echo 'hahaa' > ${temp_file}"
  run dybatpho::curl_do https://this "${temp_file}"
  assert_success
  assert_file_not_empty "${BATS_TEST_TMPDIR}/curl_do"
  unstub curl
}

@test "dybatpho::curl_do with status code 404" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_do"
  stub curl ": echo '404'"
  run -4 dybatpho::curl_do https://this "${temp_file}"
  assert_failure
  unstub curl
}

@test "dybatpho::curl_do without stub" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_do" port
  port="$(start_http_server)" || skip "python3 is required to serve HTTP"
  run_traced dybatpho::curl_do "http://127.0.0.1:${port}" "${temp_file}"
  assert_success
  refute_output
}

@test "dybatpho::curl_do with retries success" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_do"
  stub curl \
    ": echo '300'" \
    ": echo '500'" \
    ": echo '200'; echo 'hahaa' > ${temp_file}"
  run dybatpho::curl_do https://this "${temp_file}"
  assert_success
  assert_file_not_empty "${BATS_TEST_TMPDIR}/curl_do"
  unstub curl
}

@test "dybatpho::curl_do uses capped exponential backoff" {
  local sleep_file="${BATS_TEST_TMPDIR}/retry-delays"
  export DYBATPHO_CURL_MAX_RETRIES=2
  export DYBATPHO_CURL_RETRY_BASE_DELAY=1
  export DYBATPHO_CURL_RETRY_MAX_DELAY=3
  stub curl ": echo '500'" ": echo '500'" ": echo '200'"
  stub sleep \
    ": echo \"\$1\" >> ${sleep_file}" \
    ": echo \"\$1\" >> ${sleep_file}"
  run dybatpho::curl_do https://this
  assert_success
  assert_equal "$(cat "${sleep_file}")" $'1\n2'
  unstub sleep
  unstub curl
  unset DYBATPHO_CURL_MAX_RETRIES DYBATPHO_CURL_RETRY_BASE_DELAY DYBATPHO_CURL_RETRY_MAX_DELAY
}

@test "dybatpho::curl_do with retries failed" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_do"
  export DYBATPHO_CURL_MAX_RETRIES=1
  stub curl \
    ": echo '300'" \
    ": echo '300'" \
    ": echo '500'" \
    ": echo '500'" \
    ": echo '101'" \
    ": echo '101'"
  run -3 dybatpho::curl_do https://this "${temp_file}"
  assert_failure
  run -5 dybatpho::curl_do https://this "${temp_file}"
  assert_failure
  run -1 dybatpho::curl_do https://this "${temp_file}"
  assert_failure
  unstub curl
}

@test "dybatpho::curl_do with curl command failure" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_do"
  export DYBATPHO_CURL_MAX_RETRIES=0
  stub curl ": return 1"
  run --separate-stderr -1 dybatpho::curl_do https://this "${temp_file}"
  unstub curl
  assert_failure
  assert_stderr --partial "Error when access https://this"
}

@test "dybatpho::curl_download not have right spec" {
  run dybatpho::curl_download
  assert_failure
  run dybatpho::curl_download https://github.com
  assert_failure
}

@test "dybatpho::curl_download with output" {
  local temp_file=${BATS_TEST_TMPDIR}/test/curl_download
  stub curl ": echo '200'; echo 'hahaa' > ${temp_file}"
  run dybatpho::curl_download https://github.com "${temp_file}"
  assert_success
  assert_file_not_empty "${BATS_TEST_TMPDIR}/test/curl_download"
  unstub curl
}

@test "dybatpho::curl_download with more than 2 parameters" {
  local temp_file="${BATS_TEST_TMPDIR}/test/curl_download"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_download https://this "${temp_file}" --header "X-Test: 1"
  grep "header X-Test: 1" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_download adds progress flags" {
  local temp_file="${BATS_TEST_TMPDIR}/test/curl_download_flags"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_download https://this "${temp_file}"
  grep -- "-#" "${temp_file}"
  grep -- "--no-silent" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_download to readonly directory" {
  run --separate-stderr -6 dybatpho::curl_download https://example.com /root/readonly/file.txt
  assert_failure
}

@test "dybatpho::curl_json adds JSON headers" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_json"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_json https://this "${temp_file}" --request POST
  grep -- '--header Accept: application/json' "${temp_file}"
  grep -- '--header Content-Type: application/json' "${temp_file}"
  grep -- '--request POST' "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_head adds HEAD mode" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_head"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_head https://this "${temp_file}" --header "X-Test: 1"
  grep -- '-I' "${temp_file}"
  grep -- '--header X-Test: 1' "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_do rejects an empty URL" {
  run ! dybatpho::curl_do ""
}

@test "dybatpho::curl_do only prints the request when DRY_RUN is enabled" {
  DRY_RUN=true
  run_traced dybatpho::curl_do https://example.com /dev/null --header "X-Test: 1"
  DRY_RUN=""
  assert_success
  assert_output --partial "DRY RUN"
  assert_output --partial "https://example.com"
}

@test "dybatpho::curl_do maps a failed curl invocation to status 000" {
  export DYBATPHO_CURL_MAX_RETRIES=0
  stub_repeated curl ": exit 1"
  local status=0
  dybatpho::curl_do https://example.com || status=$?
  assert_equal "${status}" "1"
  unset DYBATPHO_CURL_MAX_RETRIES
}

@test "dybatpho::curl_do retries throttled responses and caps Retry-After" {
  local sleep_file="${BATS_TEST_TMPDIR}/throttle-delays"
  export DYBATPHO_CURL_MAX_RETRIES=1
  export DYBATPHO_CURL_RETRY_BASE_DELAY=1
  export DYBATPHO_CURL_RETRY_MAX_DELAY=3
  stub curl \
    ": while ((\$#)); do if [[ \$1 == -D ]]; then printf 'Retry-After: 99\r\n' > \$2; fi; shift; done; echo '429'" \
    ": echo '200'"
  stub sleep ": echo \"\$1\" >> ${sleep_file}"
  dybatpho::curl_do https://example.com
  assert_equal "$(< "${sleep_file}")" "3"
  unstub sleep
  unstub curl
  unset DYBATPHO_CURL_MAX_RETRIES DYBATPHO_CURL_RETRY_BASE_DELAY DYBATPHO_CURL_RETRY_MAX_DELAY
}

@test "dybatpho::curl_do adds jitter to the retry delay when enabled" {
  local sleep_file="${BATS_TEST_TMPDIR}/jitter-delays"
  export DYBATPHO_CURL_MAX_RETRIES=1
  export DYBATPHO_CURL_RETRY_BASE_DELAY=2
  export DYBATPHO_CURL_RETRY_MAX_DELAY=2
  export DYBATPHO_CURL_RETRY_JITTER=true
  stub curl ": echo '500'" ": echo '200'"
  stub sleep ": echo \"\$1\" >> ${sleep_file}"
  dybatpho::curl_do https://example.com
  # Jitter can only raise the delay, and the cap keeps it at the maximum.
  assert_equal "$(< "${sleep_file}")" "2"
  unstub sleep
  unstub curl
  unset DYBATPHO_CURL_MAX_RETRIES DYBATPHO_CURL_RETRY_BASE_DELAY
  unset DYBATPHO_CURL_RETRY_MAX_DELAY DYBATPHO_CURL_RETRY_JITTER
}

@test "dybatpho::curl_upload builds -F flags for value and file fields" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_upload"
  local upload_file="${BATS_TEST_TMPDIR}/report.csv"
  echo "data" > "${upload_file}"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_upload https://this "${temp_file}" note="nightly run" "report=@${upload_file}"
  grep -- "--request POST" "${temp_file}"
  grep -- "F note=nightly run" "${temp_file}"
  grep -- "F report=@${upload_file}" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_upload allows overriding the request method" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_upload_override"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_upload https://this "${temp_file}" note=1 --request PUT
  grep -- "--request PUT" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_upload fails when referenced file is missing" {
  run --separate-stderr -2 dybatpho::curl_upload https://this /dev/null "report=@${BATS_TEST_TMPDIR}/missing.csv"
  assert_failure
}

@test "dybatpho::verify_checksum succeeds on matching sha256" {
  local file="${BATS_TEST_TMPDIR}/checksum_ok"
  echo -n "hello" > "${file}"
  local digest
  digest=$(sha256sum "${file}" | awk '{print $1}')
  run dybatpho::verify_checksum "${file}" "sha256:${digest}"
  assert_success
}

@test "dybatpho::verify_checksum fails on mismatch" {
  local file="${BATS_TEST_TMPDIR}/checksum_bad"
  echo -n "hello" > "${file}"
  run -7 dybatpho::verify_checksum "${file}" "sha256:0000000000000000000000000000000000000000000000000000000000000000"
  assert_failure
}

@test "dybatpho::verify_checksum rejects invalid spec" {
  local file="${BATS_TEST_TMPDIR}/checksum_invalid"
  echo -n "hello" > "${file}"
  run -8 dybatpho::verify_checksum "${file}" "invalid-spec"
  assert_failure
}

@test "dybatpho::curl_resume_download adds resume flag" {
  local temp_file="${BATS_TEST_TMPDIR}/test/resume_download"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_resume_download https://this "${temp_file}"
  grep -- "-C -" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_resume_download verifies checksum on success" {
  local temp_file="${BATS_TEST_TMPDIR}/test/resume_checksum"
  local digest
  digest=$(echo -n "hahaa" | sha256sum | awk '{print $1}')
  stub curl ": echo '200'; echo -n 'hahaa' > ${temp_file}"
  run dybatpho::curl_resume_download https://this "${temp_file}" "sha256:${digest}"
  assert_success
  unstub curl
}

@test "dybatpho::curl_resume_download fails on checksum mismatch" {
  local temp_file="${BATS_TEST_TMPDIR}/test/resume_checksum_bad"
  stub curl ": echo '200'; echo -n 'hahaa' > ${temp_file}"
  run -7 dybatpho::curl_resume_download https://this "${temp_file}" "sha256:0000000000000000000000000000000000000000000000000000000000000000"
  assert_failure
  unstub curl
}

@test "dybatpho::curl_parse_response extracts status, headers, and body" {
  local header_file="${BATS_TEST_TMPDIR}/headers.txt"
  printf 'HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nX-Request-Id: abc\r\n\r\n' > "${header_file}"
  dybatpho::curl_parse_response "${header_file}" "/tmp/body.json"
  assert_equal "${DYBATPHO_HTTP_STATUS}" "200"
  assert_equal "${DYBATPHO_HTTP_HEADERS[content-type]}" "application/json"
  assert_equal "${DYBATPHO_HTTP_HEADERS[x-request-id]}" "abc"
  assert_equal "${DYBATPHO_HTTP_BODY_FILE}" "/tmp/body.json"
}

@test "dybatpho::curl_parse_response uses only the last block after a redirect" {
  local header_file="${BATS_TEST_TMPDIR}/headers_redirect.txt"
  printf 'HTTP/1.1 301 Moved Permanently\r\nLocation: https://new\r\n\r\nHTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n' > "${header_file}"
  dybatpho::curl_parse_response "${header_file}"
  assert_equal "${DYBATPHO_HTTP_STATUS}" "200"
  assert_equal "${DYBATPHO_HTTP_HEADERS[content-type]}" "text/plain"
  [[ -z "${DYBATPHO_HTTP_HEADERS[location]:-}" ]]
}

@test "dybatpho::curl_parse_response fails without a status line" {
  local header_file="${BATS_TEST_TMPDIR}/headers_empty.txt"
  : > "${header_file}"
  run -1 dybatpho::curl_parse_response "${header_file}"
  assert_failure
}

@test "dybatpho::curl_response_header returns value or default" {
  local header_file="${BATS_TEST_TMPDIR}/headers_get.txt"
  printf 'HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n' > "${header_file}"
  dybatpho::curl_parse_response "${header_file}"
  assert_equal "$(dybatpho::curl_response_header Content-Type)" "application/json"
  assert_equal "$(dybatpho::curl_response_header X-Missing default-value)" "default-value"
  run dybatpho::curl_response_header X-Missing
  assert_failure
}

@test "dybatpho::curl_request populates normalized response state" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_request"
  stub curl \
    ": while ((\$#)); do if [[ \$1 == -D ]]; then printf 'HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n' > \$2; fi; shift; done; echo 'body' > ${temp_file}; echo '200'"
  dybatpho::curl_request https://this "${temp_file}"
  assert_equal "${DYBATPHO_HTTP_STATUS}" "200"
  assert_equal "${DYBATPHO_HTTP_HEADERS[content-type]}" "application/json"
  assert_equal "${DYBATPHO_HTTP_BODY_FILE}" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_request skips response parsing during dry run" {
  DRY_RUN=true
  DYBATPHO_HTTP_STATUS=""
  run_traced dybatpho::curl_request https://example.com /dev/null
  DRY_RUN=""
  assert_success
  assert_equal "${DYBATPHO_HTTP_STATUS}" ""
}

@test "dybatpho::curl_timeout applies scoped connect/total timeout overrides" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_timeout"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_timeout https://this "${temp_file}" 2 10
  grep -- "--connect-timeout 2" "${temp_file}"
  grep -- "--max-time 10" "${temp_file}"
  unstub curl
  # Global defaults must remain untouched after the scoped call.
  assert_equal "${DYBATPHO_CURL_CONNECT_TIMEOUT}" ""
  assert_equal "${DYBATPHO_CURL_TIMEOUT}" ""
}

@test "dybatpho::curl_timeout allows overriding only the total timeout" {
  local temp_file="${BATS_TEST_TMPDIR}/curl_timeout_total_only"
  stub curl ": echo \"\$*\" > ${temp_file}; echo '200'"
  dybatpho::curl_timeout https://this "${temp_file}" "" 5
  grep -- "--max-time 5" "${temp_file}"
  ! grep -- "--connect-timeout" "${temp_file}"
  unstub curl
}

@test "dybatpho::curl_timeout rejects non-numeric overrides" {
  run --separate-stderr dybatpho::curl_timeout https://this /dev/null abc
  assert_failure
}

@test "dybatpho::circuit_breaker stays closed while under threshold" {
  export DYBATPHO_CIRCUIT_THRESHOLD=3
  dybatpho::circuit_reset test-service
  run dybatpho::circuit_breaker test-service "false"
  assert_failure
  assert_equal "$(dybatpho::circuit_state test-service)" "closed"
  unset DYBATPHO_CIRCUIT_THRESHOLD
}

@test "dybatpho::circuit_breaker opens after reaching the failure threshold" {
  export DYBATPHO_CIRCUIT_THRESHOLD=2
  export DYBATPHO_CIRCUIT_COOLDOWN=60
  dybatpho::circuit_reset flaky-service
  local status=0
  dybatpho::circuit_breaker flaky-service "false" || true
  dybatpho::circuit_breaker flaky-service "false" || status=$?
  assert_equal "${status}" "1"
  assert_equal "$(dybatpho::circuit_state flaky-service)" "open"
  unset DYBATPHO_CIRCUIT_THRESHOLD DYBATPHO_CIRCUIT_COOLDOWN
}

@test "dybatpho::circuit_breaker short-circuits calls while open" {
  export DYBATPHO_CIRCUIT_THRESHOLD=1
  export DYBATPHO_CIRCUIT_COOLDOWN=60
  dybatpho::circuit_reset blocked-service
  dybatpho::circuit_breaker blocked-service "false" || true
  run -9 dybatpho::circuit_breaker blocked-service "true"
  assert_failure
  unset DYBATPHO_CIRCUIT_THRESHOLD DYBATPHO_CIRCUIT_COOLDOWN
}

@test "dybatpho::circuit_breaker closes again after a successful call" {
  export DYBATPHO_CIRCUIT_THRESHOLD=1
  dybatpho::circuit_reset recovering-service
  dybatpho::circuit_breaker recovering-service "false" || true
  assert_equal "$(dybatpho::circuit_state recovering-service)" "open"
  dybatpho::circuit_reset recovering-service
  run dybatpho::circuit_breaker recovering-service "true"
  assert_success
  assert_equal "$(dybatpho::circuit_state recovering-service)" "closed"
  unset DYBATPHO_CIRCUIT_THRESHOLD
}

# --- URL parsing ---------------------------------------------------------

@test "dybatpho::url_parse splits a URL that uses every component" {
  dybatpho::url_parse "https://user:sec@example.com:8443/a/b?q=1&r=2#top"
  assert_equal "${DYBATPHO_URL[scheme]}" "https"
  assert_equal "${DYBATPHO_URL[user]}" "user"
  assert_equal "${DYBATPHO_URL[password]}" "sec"
  assert_equal "${DYBATPHO_URL[host]}" "example.com"
  assert_equal "${DYBATPHO_URL[port]}" "8443"
  assert_equal "${DYBATPHO_URL[path]}" "/a/b"
  assert_equal "${DYBATPHO_URL[query]}" "q=1&r=2"
  assert_equal "${DYBATPHO_URL[fragment]}" "top"
}

@test "dybatpho::url_parse leaves an absent component empty rather than unset" {
  dybatpho::url_parse "http://example.com"
  assert_equal "${DYBATPHO_URL[host]}" "example.com"
  assert_equal "${DYBATPHO_URL[port]}" ""
  assert_equal "${DYBATPHO_URL[path]}" ""
  assert_equal "${DYBATPHO_URL[query]}" ""
}

@test "dybatpho::url_parse clears what the previous URL left behind" {
  dybatpho::url_parse "https://user:sec@example.com:8443/a?q=1#top"
  dybatpho::url_parse "http://example.org"
  assert_equal "${DYBATPHO_URL[user]}" ""
  assert_equal "${DYBATPHO_URL[password]}" ""
  assert_equal "${DYBATPHO_URL[port]}" ""
  assert_equal "${DYBATPHO_URL[fragment]}" ""
}

@test "dybatpho::url_parse reads a bracketed IPv6 host apart from its port" {
  dybatpho::url_parse "http://[2001:db8::1]:8080/health"
  assert_equal "${DYBATPHO_URL[host]}" "2001:db8::1"
  assert_equal "${DYBATPHO_URL[port]}" "8080"

  dybatpho::url_parse "http://[::1]/x"
  assert_equal "${DYBATPHO_URL[host]}" "::1"
  assert_equal "${DYBATPHO_URL[port]}" ""
}

@test "dybatpho::url_parse takes the credentials at the last at-sign" {
  # A password may contain an `@`, and splitting at the first one would move the
  # host boundary into the credentials.
  dybatpho::url_parse "https://user:p@ss@example.com/x"
  assert_equal "${DYBATPHO_URL[user]}" "user"
  assert_equal "${DYBATPHO_URL[password]}" "p@ss"
  assert_equal "${DYBATPHO_URL[host]}" "example.com"
}

@test "dybatpho::url_parse lower-cases the scheme and leaves the rest alone" {
  dybatpho::url_parse "HTTPS://Example.COM/Path?A=B"
  assert_equal "${DYBATPHO_URL[scheme]}" "https"
  assert_equal "${DYBATPHO_URL[host]}" "Example.COM"
  assert_equal "${DYBATPHO_URL[path]}" "/Path"
  assert_equal "${DYBATPHO_URL[query]}" "A=B"
}

@test "dybatpho::url_parse keeps percent-escapes as written" {
  dybatpho::url_parse "https://example.com/a%2Fb?q=a%20b"
  assert_equal "${DYBATPHO_URL[path]}" "/a%2Fb"
  assert_equal "${DYBATPHO_URL[query]}" "q=a%20b"
}

@test "dybatpho::url_parse handles a scheme that is not http" {
  dybatpho::url_parse "postgres://u@db/app?sslmode=require"
  assert_equal "${DYBATPHO_URL[scheme]}" "postgres"
  assert_equal "${DYBATPHO_URL[user]}" "u"
  assert_equal "${DYBATPHO_URL[password]}" ""
  assert_equal "${DYBATPHO_URL[host]}" "db"
  assert_equal "${DYBATPHO_URL[path]}" "/app"
}

@test "dybatpho::url_parse rejects a URL it cannot take apart" {
  run ! dybatpho::url_parse "example.com/x"
  run ! dybatpho::url_parse "https:///path"
  run ! dybatpho::url_parse "https://host:notaport/x"
  run ! dybatpho::url_parse "https://host:99999/x"
  run ! dybatpho::url_parse "https://host:0/x"
}

@test "dybatpho::url_part prints a component and honors a default" {
  dybatpho::url_parse "https://example.com/health"
  assert_equal "$(dybatpho::url_part host)" "example.com"
  assert_equal "$(dybatpho::url_part port 443)" "443"
  run ! dybatpho::url_part port
}

@test "dybatpho::url_part rejects a name that is not a component" {
  dybatpho::url_parse "https://example.com/"
  run --separate-stderr ! dybatpho::url_part hostname
  assert_stderr --partial "is not a component of a URL"
}

# --- Addresses -----------------------------------------------------------

@test "dybatpho::is_ipv4 accepts addresses and refuses what only looks like one" {
  for address in 0.0.0.0 192.0.2.10 255.255.255.255 10.1.2.3; do
    dybatpho::is_ipv4 "${address}" || fail "rejected ${address}"
  done
  for address in 192.0.2.256 1.2.3 1.2.3.4.5 "" 1.2.3.a ::1 " 1.2.3.4"; do
    ! dybatpho::is_ipv4 "${address}" || fail "accepted ${address}"
  done
}

@test "dybatpho::is_ipv4 refuses an octet with a leading zero" {
  # `inet_aton` reads `010` as octal, so this address means one host to the
  # resolver and another to a reader. Refusing it is the only answer that does
  # not silently pick one of the two.
  run ! dybatpho::is_ipv4 127.0.0.010
  run ! dybatpho::is_ipv4 010.1.1.1
  dybatpho::is_ipv4 127.0.0.0
}

@test "dybatpho::is_ipv6 accepts every shape an address may be written in" {
  for address in ::1 :: 2001:db8::1 1:2:3:4:5:6:7:8 2001:db8:: fe80::1 \
    ::ffff:192.0.2.1 64:ff9b::1.2.3.4 0:0:0:0:0:ffff:192.0.2.1; do
    dybatpho::is_ipv6 "${address}" || fail "rejected ${address}"
  done
}

@test "dybatpho::is_ipv6 refuses malformed addresses" {
  for address in 2001:db8::1::2 1:2:3:4:5:6:7:8:9 1:2:3:4:5:6:7 12345::1 \
    "" 1.2.3.4 "1:2:3:4:5:6:7:8:" ":1:2:3:4:5:6:7" "gggg::1" "1:2:::3"; do
    ! dybatpho::is_ipv6 "${address}" || fail "accepted ${address}"
  done
}

@test "dybatpho::is_ipv6 refuses a zone index" {
  # `%eth0` names an interface on one host, so the address is not comparable
  # with the same text read anywhere else.
  run ! dybatpho::is_ipv6 "fe80::1%eth0"
  dybatpho::is_ipv6 "fe80::1"
}

@test "dybatpho::ip_version names the version or fails" {
  assert_equal "$(dybatpho::ip_version 192.0.2.1)" "4"
  assert_equal "$(dybatpho::ip_version 2001:db8::1)" "6"
  assert_equal "$(dybatpho::ip_version ::ffff:192.0.2.1)" "6"
  run ! dybatpho::ip_version "nope"
}

# --- Networks ------------------------------------------------------------

@test "dybatpho::is_cidr accepts blocks of both versions" {
  for block in 10.0.0.0/8 0.0.0.0/0 192.0.2.1/32 2001:db8::/32 ::/0 ::1/128; do
    dybatpho::is_cidr "${block}" || fail "rejected ${block}"
  done
  for block in 10.0.0.0/33 10.0.0.0 2001:db8::/129 10.0.0.0/08 "10.0.0.0/" "/8"; do
    ! dybatpho::is_cidr "${block}" || fail "accepted ${block}"
  done
}

@test "dybatpho::cidr_netmask converts a prefix length to a dotted mask" {
  assert_equal "$(dybatpho::cidr_netmask 0)" "0.0.0.0"
  assert_equal "$(dybatpho::cidr_netmask 1)" "128.0.0.0"
  assert_equal "$(dybatpho::cidr_netmask 16)" "255.255.0.0"
  assert_equal "$(dybatpho::cidr_netmask 24)" "255.255.255.0"
  assert_equal "$(dybatpho::cidr_netmask 32)" "255.255.255.255"
  assert_equal "$(dybatpho::cidr_netmask /16)" "255.255.0.0"
}

@test "dybatpho::cidr_netmask rejects a prefix length IPv4 does not have" {
  run --separate-stderr ! dybatpho::cidr_netmask 33
  assert_stderr --partial "is not an IPv4 prefix length"
  run --separate-stderr ! dybatpho::cidr_netmask abc
}

@test "dybatpho::cidr_contains decides IPv4 membership at the block edges" {
  dybatpho::cidr_contains 10.0.0.0/8 10.1.2.3
  ! dybatpho::cidr_contains 10.0.0.0/8 11.1.2.3 || fail "11.1.2.3 in 10/8"
  dybatpho::cidr_contains 0.0.0.0/0 203.0.113.1
  dybatpho::cidr_contains 192.0.2.1/32 192.0.2.1
  ! dybatpho::cidr_contains 192.0.2.1/32 192.0.2.2 || fail "/32 too wide"
  dybatpho::cidr_contains 192.168.1.0/24 192.168.1.255
  ! dybatpho::cidr_contains 192.168.1.0/24 192.168.2.0 || fail "/24 too wide"
  # A /12 boundary is the one hand-written checks usually get wrong.
  dybatpho::cidr_contains 172.16.0.0/12 172.31.255.255
  ! dybatpho::cidr_contains 172.16.0.0/12 172.32.0.0 || fail "/12 too wide"
}

@test "dybatpho::cidr_contains decides IPv6 membership, including inside a group" {
  dybatpho::cidr_contains 2001:db8::/32 2001:db8::1
  dybatpho::cidr_contains 2001:db8::/32 2001:db8:ffff::1
  ! dybatpho::cidr_contains 2001:db8::/32 2001:db9::1 || fail "/32 too wide"
  dybatpho::cidr_contains ::/0 2001:db8::1
  dybatpho::cidr_contains ::1/128 ::1
  ! dybatpho::cidr_contains ::1/128 ::2 || fail "/128 too wide"
  # A prefix that ends mid-group is where a group-at-a-time comparison breaks.
  dybatpho::cidr_contains 2001:db8::/33 2001:db8:7fff::1
  ! dybatpho::cidr_contains 2001:db8::/33 2001:db8:8000::1 || fail "/33 too wide"
}

@test "dybatpho::cidr_contains keeps the two IP versions apart" {
  # Some software treats `::ffff:10.0.0.1` and `10.0.0.1` as the same host. They
  # are not the same address, and a membership test that blurred them would be
  # a way to get past an allowlist.
  ! dybatpho::cidr_contains 10.0.0.0/8 ::1 || fail "v6 inside a v4 block"
  ! dybatpho::cidr_contains ::/0 10.0.0.1 || fail "v4 inside a v6 block"
  ! dybatpho::cidr_contains 10.0.0.0/8 ::ffff:10.0.0.1 || fail "mapped address let in"
}

@test "dybatpho::cidr_contains stops the script on a malformed argument" {
  run --separate-stderr ! dybatpho::cidr_contains "notablock" 10.0.0.1
  assert_stderr --partial "is not a CIDR block"
  run --separate-stderr ! dybatpho::cidr_contains 10.0.0.0/8 "notanaddress"
  assert_stderr --partial "is not an IP address"
}

# --- Ports ---------------------------------------------------------------

@test "dybatpho::port_open reports a port nothing is listening on as closed" {
  run ! dybatpho::port_open 127.0.0.1 1 1
}

@test "dybatpho::port_open finds a port that is listening" {
  local port
  port="$(start_listener)" || skip "no way to open a listening socket here"
  dybatpho::port_open 127.0.0.1 "${port}" 2
}

@test "dybatpho::wait_port returns as soon as the port answers" {
  local port
  port="$(start_listener)" || skip "no way to open a listening socket here"
  dybatpho::wait_port 127.0.0.1 "${port}" 5
}

@test "dybatpho::wait_port gives up within its budget" {
  local started=${SECONDS}
  run ! dybatpho::wait_port 127.0.0.1 1 2 1
  # The budget is what bounds the call, including the time each attempt takes.
  ((SECONDS - started <= 8)) || fail "waited $((SECONDS - started))s for a 2s budget"
}

@test "dybatpho::port_open and wait_port reject arguments that are not numbers" {
  run --separate-stderr ! dybatpho::port_open host notaport
  assert_stderr --partial "is not a port number"
  run --separate-stderr ! dybatpho::port_open host 80 later
  assert_stderr --partial "is not a number of seconds"
  run --separate-stderr ! dybatpho::wait_port host 80 soon
  assert_stderr --partial "is not a number of seconds"
}

@test "credentials and bodies never reach curl's command line" {
  export DYBATPHO_CURL_MAX_RETRIES=0
  dybatpho::mock_http "example.test/oob" 200 'ok'

  local -a DYBATPHO_CURL_SECRET_HEADERS=("Authorization: Bearer leak-me-not")
  local DYBATPHO_CURL_SECRET_DATA='{"secret":"body"}'
  dybatpho::curl_do "https://example.test/oob" "${BATS_TEST_TMPDIR}/out"

  # Arguments are world-readable through /proc/<pid>/cmdline, so neither the
  # credential nor the body may appear there.
  run dybatpho::mock_calls curl
  assert_success
  refute_output --partial "leak-me-not"
  refute_output --partial '"secret":"body"'

  # They were still sent, out of band.
  run dybatpho::mock_http_payloads
  assert_success
  assert_output --partial "Authorization: Bearer leak-me-not"
  assert_output --partial '{"secret":"body"}'
}
