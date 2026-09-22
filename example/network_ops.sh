#!/usr/bin/env bash
# @file network_ops.sh
# @brief Example showing network utilities
# @description Demonstrates dybatpho::curl_do, curl_download, curl_json,
#   curl_head, curl_upload, curl_resume_download, verify_checksum,
#   curl_request/curl_parse_response, curl_timeout, and circuit_breaker
#
#   Every request below is served by a stub, so the example runs offline and
#   produces the same output on every machine. Only the transport is faked:
#   retry, header parsing, checksum verification and the circuit breaker are the
#   real code paths.
SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=init.sh
. "${SCRIPTDIR}/../init.sh" --modules network

dybatpho::register_common_handlers

# @description Install a `curl` stub on PATH so the example needs no network.
#   `dybatpho::curl_do` runs `command curl`, which deliberately bypasses shell
#   functions, so the stub has to be a real executable earlier on PATH.
#
#   The stub reproduces the three things the module asks curl for: the body at
#   the path after `-o`, the response headers at the path after `-D`, and the
#   status code on stdout via `-w '%{http_code}'`.
function _install_curl_stub {
  local stub_dir
  dybatpho::create_temp_dir stub_dir "curl-stub"

  cat > "${stub_dir}/curl" << 'STUB'
#!/usr/bin/env bash
output=""
header_file=""
prev=""
url="${!#}"
for arg in "$@"; do
  case "${prev}" in
    -o) output="${arg}" ;;
    -D) header_file="${arg}" ;;
  esac
  prev="${arg}"
done

content_type='text/plain'
case "${url}" in
  *api.github.com*)
    content_type='application/json'
    body='{"name":"dybatpho","stargazers_count":42,"license":{"key":"wtfpl"}}'
    ;;
  *httpbin.org/post*)
    content_type='application/json'
    body='{"form":{"note":"nightly run"},"files":{"report":"metric,value"}}'
    ;;
  *hello.txt) body='hello dybatpho' ;;
  *) body='<!doctype html><title>Example Domain</title>' ;;
esac

if [[ -n "${header_file}" ]]; then
  printf 'HTTP/2 200\r\ncontent-type: %s\r\ncontent-length: %s\r\n\r\n' \
    "${content_type}" "${#body}" > "${header_file}"
fi
if [[ -n "${output}" && "${output}" != "/dev/null" ]]; then
  printf '%s' "${body}" > "${output}"
fi
printf '200'
STUB

  chmod +x "${stub_dir}/curl"
  export PATH="${stub_dir}:${PATH}"
}

function _demo_head_request {
  dybatpho::header "HEAD REQUEST"
  local headers_file
  dybatpho::create_temp headers_file ".txt"
  if dybatpho::curl_head "https://example.com" "${headers_file}"; then
    dybatpho::info "Saved response headers to ${headers_file}"
    dybatpho::show_file "${headers_file}"
  else
    dybatpho::warn "HEAD request failed"
  fi
}

function _demo_json_request {
  dybatpho::header "JSON REQUEST"
  local json_file
  dybatpho::create_temp json_file ".json"
  if dybatpho::curl_json "https://api.github.com/repos/dynamotn/dybatpho" "${json_file}"; then
    dybatpho::info "Fetched JSON response to ${json_file}"
    dybatpho::show_file "${json_file}"
  else
    dybatpho::warn "JSON request failed"
  fi
}

function _demo_upload {
  dybatpho::header "MULTIPART UPLOAD"
  local report_file response_file
  dybatpho::create_temp report_file ".csv"
  dybatpho::create_temp response_file ".json"
  printf 'metric,value\nrequests,42\n' > "${report_file}"
  if dybatpho::curl_upload "https://httpbin.org/post" "${response_file}" \
    note="nightly run" "report=@${report_file}"; then
    dybatpho::info "Uploaded ${report_file}, response saved to ${response_file}"
  else
    dybatpho::warn "Upload failed"
  fi
}

function _demo_resume_download_with_checksum {
  dybatpho::header "RESUMABLE DOWNLOAD + CHECKSUM"
  local dst_file expected_checksum
  dybatpho::create_temp dst_file ".txt"
  # A checksum computed ahead of time for the expected content, e.g. from a manifest.
  expected_checksum="sha256:$(printf 'hello dybatpho' | sha256sum | awk '{print $1}')"
  if dybatpho::curl_resume_download "https://example.com/hello.txt" "${dst_file}" "${expected_checksum}"; then
    dybatpho::info "Downloaded and verified ${dst_file}"
  else
    dybatpho::warn "Resumable download or checksum verification failed"
  fi
}

function _demo_normalized_response {
  dybatpho::header "NORMALIZED RESPONSE PARSING"
  local body_file
  dybatpho::create_temp body_file ".json"
  if dybatpho::curl_request "https://api.github.com/repos/dynamotn/dybatpho" "${body_file}"; then
    dybatpho::info "Status: ${DYBATPHO_HTTP_STATUS}"
    dybatpho::info "Content-Type: $(dybatpho::curl_response_header content-type unknown)"
  else
    dybatpho::warn "Request failed with status: ${DYBATPHO_HTTP_STATUS}"
  fi
}

function _demo_per_request_timeout {
  dybatpho::header "PER-REQUEST TIMEOUT"
  # Override connect/total timeouts for this single call only; global
  # DYBATPHO_CURL_CONNECT_TIMEOUT/DYBATPHO_CURL_TIMEOUT stay untouched.
  if dybatpho::curl_timeout "https://example.com" /dev/null 2 5; then
    dybatpho::info "Request completed within the scoped timeout"
  else
    dybatpho::warn "Request failed or exceeded the scoped timeout"
  fi
}

function _demo_circuit_breaker {
  dybatpho::header "CIRCUIT BREAKER"
  export DYBATPHO_CIRCUIT_THRESHOLD=2
  export DYBATPHO_CIRCUIT_COOLDOWN=30
  local service="example-service"
  dybatpho::circuit_reset "${service}"
  # Simulate two consecutive failures to open the circuit, then show that a
  # third attempt is short-circuited without contacting the service.
  dybatpho::circuit_breaker "${service}" "false" || true
  dybatpho::circuit_breaker "${service}" "false" || true
  if dybatpho::circuit_breaker "${service}" "true"; then
    dybatpho::info "Call succeeded"
  else
    dybatpho::warn "Circuit '${service}' is $(dybatpho::circuit_state "${service}"); call was skipped"
  fi
  unset DYBATPHO_CIRCUIT_THRESHOLD DYBATPHO_CIRCUIT_COOLDOWN
}

function _main {
  _install_curl_stub
  _demo_head_request
  _demo_json_request
  _demo_upload
  _demo_resume_download_with_checksum
  _demo_normalized_response
  _demo_per_request_timeout
  _demo_circuit_breaker
  dybatpho::success "Network operations demo complete"
}

_main "$@"
