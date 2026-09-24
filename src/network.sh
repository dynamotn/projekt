#!/usr/bin/env bash
# @file network.sh
# @brief Utilities for network
# @description
#   This module contains functions to work with network connection, downloads,
#   JSON-oriented requests, and HEAD requests. It also provides multipart
#   uploads, resumable downloads with checksum verification, normalized
#   response parsing (status/headers/body), per-request timeouts, and an
#   in-memory circuit breaker.
#
#   Alongside the HTTP client it carries the primitives a script reaches for
#   before making a request at all: splitting a URL into its parts, deciding
#   whether a string is an address or a network, whether an address is inside
#   one, and whether a port is accepting connections yet. These are the checks
#   that otherwise get written inline as a regex that nearly works — the kind
#   that accepts `192.0.2.256`, or reads `127.0.0.010` as a different host than
#   the resolver does.
: "${DYBATPHO_DIR:?DYBATPHO_DIR must be set. Please source dybatpho/init.sh before other scripts from dybatpho.}"

# @env DYBATPHO_CURL_MAX_RETRIES number Max number of retry attempts when `dybatpho::curl_do` retries a request
# @env DYBATPHO_CURL_RETRY_BASE_DELAY number Initial retry delay in seconds (default `2`)
# @env DYBATPHO_CURL_RETRY_MAX_DELAY number Maximum retry delay in seconds (default `30`)
# @env DYBATPHO_CURL_RETRY_JITTER bool Add up to one base delay of random jitter
# @env DYBATPHO_CURL_CONNECT_TIMEOUT number Optional curl connection timeout in seconds
# @env DYBATPHO_CURL_TIMEOUT number Optional curl total timeout in seconds
# @env DYBATPHO_CIRCUIT_THRESHOLD number Consecutive failures before `dybatpho::circuit_breaker` opens a circuit (default `5`)
# @env DYBATPHO_CIRCUIT_COOLDOWN number Seconds an open circuit waits before allowing a trial request (default `30`)
# @env DYBATPHO_PORT_TIMEOUT number Seconds `dybatpho::port_open` waits for a connection (default `5`)
# @env DYBATPHO_WAIT_PORT_TIMEOUT number Seconds `dybatpho::wait_port` keeps trying before giving up (default `30`)
# @env DYBATPHO_WAIT_PORT_INTERVAL number Seconds `dybatpho::wait_port` sleeps between attempts (default `1`)
DYBATPHO_CURL_MAX_RETRIES=${DYBATPHO_CURL_MAX_RETRIES:-5}
DYBATPHO_CURL_RETRY_BASE_DELAY=${DYBATPHO_CURL_RETRY_BASE_DELAY:-2}
DYBATPHO_CURL_RETRY_MAX_DELAY=${DYBATPHO_CURL_RETRY_MAX_DELAY:-30}
DYBATPHO_CURL_RETRY_JITTER=${DYBATPHO_CURL_RETRY_JITTER:-false}
DYBATPHO_CURL_CONNECT_TIMEOUT=${DYBATPHO_CURL_CONNECT_TIMEOUT:-}
DYBATPHO_CURL_TIMEOUT=${DYBATPHO_CURL_TIMEOUT:-}
DYBATPHO_CIRCUIT_THRESHOLD=${DYBATPHO_CIRCUIT_THRESHOLD:-5}
DYBATPHO_CIRCUIT_COOLDOWN=${DYBATPHO_CIRCUIT_COOLDOWN:-30}
DYBATPHO_PORT_TIMEOUT=${DYBATPHO_PORT_TIMEOUT:-5}
DYBATPHO_WAIT_PORT_TIMEOUT=${DYBATPHO_WAIT_PORT_TIMEOUT:-30}
DYBATPHO_WAIT_PORT_INTERVAL=${DYBATPHO_WAIT_PORT_INTERVAL:-1}

# Normalized state populated by `dybatpho::curl_parse_response`/`dybatpho::curl_request`.
declare -gA DYBATPHO_HTTP_HEADERS=()
DYBATPHO_HTTP_STATUS=""
DYBATPHO_HTTP_BODY_FILE=""

# Request material that must not reach `curl`'s command line. A process's
# arguments are world-readable through `/proc/<pid>/cmdline`, which is what
# `ps auxww` prints, so an `Authorization:` header passed as `--header` is
# readable by every other account on the host for as long as the request runs.
#
# `dybatpho::curl_do` moves anything listed here out of the argument vector: the
# headers into a private config file that `curl` reads with `--config`, and the
# body onto `curl`'s standard input. Both are declared with `local -a` /`local`
# by the caller, so Bash's dynamic scoping makes them visible to `curl_do` and
# removes them again when the caller returns.
# @env DYBATPHO_CURL_SECRET_HEADERS array Headers to pass out of band, as `Name: value`
declare -ga DYBATPHO_CURL_SECRET_HEADERS=()
# @env DYBATPHO_CURL_SECRET_DATA string Request body to pass on stdin instead of in an argument
DYBATPHO_CURL_SECRET_DATA=""

# Per-key in-memory state used by `dybatpho::circuit_breaker`.
declare -gA DYBATPHO_CIRCUIT_FAILURES=()
declare -gA DYBATPHO_CIRCUIT_OPENED_AT=()

# Parsed components of the last URL, populated by `dybatpho::url_parse`.
declare -gA DYBATPHO_URL=()

# Split a URL into scheme, authority, path, query, and fragment. The authority
# is taken apart separately, because its own grammar is the awkward part.
__DYBATPHO_URL_REGEX='^([A-Za-z][A-Za-z0-9+.-]*)://([^/?#]*)([^?#]*)(\?([^#]*))?(#(.*))?$'

# One group of an IPv6 address: one to four hexadecimal digits.
__DYBATPHO_IPV6_GROUP_REGEX='^[0-9A-Fa-f]{1,4}$'

#######################################
# @description Escape a value for a double-quoted `curl` config parameter.
#   `curl` reads a config file as `name = "value"`, where the value takes
#   backslash escapes, so a backslash or a quote inside a header has to be
#   escaped or it ends the value early.
# @arg $1 string Raw value
# @stdout The escaped value, without its surrounding quotes
#######################################
function __dybatpho_network_config_escape {
  local value="${1-}"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\t'/\\t}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\n'/\\n}"
  printf '%s' "${value}"
}

#######################################
# @description Write the secret headers of the current request into a private
#   config file for `curl --config`.
#
#   The file is created under `umask 077` before anything is written to it, so
#   the credential is never on disk in a mode another account could read, and it
#   is removed as soon as the request is over. The path is an argument, which is
#   public; the contents are not.
# @arg $1 string Name of the variable receiving the config file path
# @set The named variable
# @exitcode 0 A config file was written, or there was nothing to write
# @exitcode 1 Stop the script when the file cannot be created
#######################################
function __dybatpho_network_secret_config {
  local __config_out_name
  dybatpho::expect_args __config_out_name -- "$@"
  local -n __config_out="${__config_out_name}"
  __config_out=""

  ((${#DYBATPHO_CURL_SECRET_HEADERS[@]})) || return 0

  local previous_umask path header
  previous_umask="$(umask)"
  umask 077
  path="$(mktemp "${TMPDIR:-/tmp}/dybatpho_curl_XXXXXXXX")" || {
    umask "${previous_umask}"
    dybatpho::die "${FUNCNAME[1]}: Unable to create a private curl config file"
  }
  umask "${previous_umask}"

  for header in "${DYBATPHO_CURL_SECRET_HEADERS[@]}"; do
    [[ -n "${header}" ]] || continue
    printf 'header = "%s"\n' "$(__dybatpho_network_config_escape "${header}")" >> "${path}"
  done

  __config_out="${path}"
}

#######################################
# @description Get description of HTTP status code
# @arg $1 string Status code
# @stdout Description of status code
#######################################
function __dybatpho_network_get_http_code {
  local code
  dybatpho::expect_args code -- "$@"

  case "${code}" in
    # kcov(disabled)
    '100') echo '100 (continue)' ;;
    '101') echo '101 (switching protocols)' ;;
    # kcov(enabled)
    '200') echo 'done' ;;
    # kcov(disabled)
    '201') echo '201 (created)' ;;
    '202') echo '202 (accepted)' ;;
    '203') echo '203 (non-authoritative information)' ;;
    '204') echo '204 (no content)' ;;
    '205') echo '205 (reset content)' ;;
    '206') echo '206 (partial content)' ;;
    '300') echo '300 (multiple choices)' ;;
    '301') echo '301 (moved permanently)' ;;
    '302') echo '302 (found)' ;;
    '303') echo '303 (see other)' ;;
    '304') echo '304 (not modified)' ;;
    '305') echo '305 (use proxy)' ;;
    '306') echo '306 (switch proxy)' ;;
    '307') echo '307 (temporary redirect)' ;;
    '400') echo '400 (bad request)' ;;
    '401') echo '401 (unauthorized)' ;;
    '402') echo '402 (payment required)' ;;
    # kcov(enabled)
    '403') echo '403 (forbidden)' ;;
    # kcov(disabled)
    '404') echo '404 (not found)' ;;
    '405') echo '405 (method not allowed)' ;;
    '406') echo '406 (not acceptable)' ;;
    '407') echo '407 (proxy authentication required)' ;;
    '408') echo '408 (request timeout)' ;;
    '409') echo '409 (conflict)' ;;
    '410') echo '410 (gone)' ;;
    '411') echo '411 (length required)' ;;
    '412') echo '412 (precondition failed)' ;;
    '413') echo '413 (request entity too large)' ;;
    '414') echo '414 (request URI too long)' ;;
    '415') echo '415 (unsupported media type)' ;;
    '416') echo '416 (requested range)' ;;
    '417') echo '417 (expectation failed)' ;;
    '418') echo "418 (I'm a teapot)" ;;
    '419') echo '419 (authentication timeout)' ;;
    '420') echo '420 (enhance your calm)' ;;
    '426') echo '426 (upgrade required)' ;;
    '428') echo '428 (precondition required)' ;;
    '429') echo '429 (too many requests)' ;;
    '431') echo '431 (request header fields too large)' ;;
    '451') echo '451 (unavailable for legal reasons)' ;;
    '500') echo '500 (internal server error)' ;;
    '501') echo '501 (not implemented)' ;;
    '502') echo '502 (bad gateway)' ;;
    '503') echo '503 (service unavailable)' ;;
    '504') echo '504 (gateway timeout)' ;;
    '505') echo '505 (HTTP version not supported)' ;;
    '506') echo '506 (variant also negotiates)' ;;
    '510') echo '510 (not extended)' ;;
    '511') echo '511 (network authentication required)' ;;
    *) echo "${code} (unknown)" ;;
      # kcov(enabled)
  esac
}

#######################################
# @description Transferring data with URL by curl
# @example
#   dybatpho::curl_do https://example.com /tmp/1
#   dybatpho::curl_do https://example.com /tmp/1 --compressed
#
# @arg $1 string URL
# @arg $2 string Location of curl output, default is `/dev/null`
# @arg $3 string Other options/arguments for curl
# @env DYBATPHO_CURL_MAX_RETRIES number Override the retry budget used around curl requests
# @exitcode 0 Transferred data
# @exitcode 1 Unknown error
# @exitcode 3 First digit of HTTP error code 3xx
# @exitcode 4 First digit of HTTP error code 4xx
# @exitcode 5 First digit of HTTP error code 5xx
# @exitcode 127 Curl isn't installed
# @tip The request body is written to the provided output file, or `/dev/null` when omitted
# @note HTTP 4xx responses are treated as completed requests and returned to the caller as exit code `4`
#######################################
function dybatpho::curl_do {
  local url
  dybatpho::expect_args url -- "$@"
  shift

  if dybatpho::is empty "${url}"; then
    return 1
  fi

  local output="/dev/null"
  if [ $# -ne 0 ]; then
    output="$1"
    shift
  fi

  if dybatpho::is true "${DRY_RUN}"; then
    dybatpho::dry_run curl -fsSL "${url}" -o "${output}" "$@"
    return 0
  fi

  dybatpho::require curl
  [[ "${DYBATPHO_CURL_MAX_RETRIES}" =~ ^[0-9]+$ ]] \
    || dybatpho::die "DYBATPHO_CURL_MAX_RETRIES must be a non-negative integer"
  [[ "${DYBATPHO_CURL_RETRY_BASE_DELAY}" =~ ^[0-9]+$ ]] \
    || dybatpho::die "DYBATPHO_CURL_RETRY_BASE_DELAY must be a non-negative integer"
  [[ "${DYBATPHO_CURL_RETRY_MAX_DELAY}" =~ ^[0-9]+$ ]] \
    || dybatpho::die "DYBATPHO_CURL_RETRY_MAX_DELAY must be a non-negative integer"

  local code="" retry_after delay attempt=0
  local header_file
  header_file=$(mktemp) || dybatpho::die "Unable to create temporary HTTP header file"

  # Credentials and request bodies stay out of the argument vector; see
  # DYBATPHO_CURL_SECRET_HEADERS above for why.
  local secret_config=""
  __dybatpho_network_secret_config secret_config
  local -a secret_args=()
  [[ -n "${secret_config}" ]] && secret_args+=(--config "${secret_config}")
  local body_on_stdin=false
  if [[ -n "${DYBATPHO_CURL_SECRET_DATA}" ]]; then
    secret_args+=(--data-binary @-)
    body_on_stdin=true
  fi
  # Keep the body path owned by the caller; only response headers are temporary.
  local __dybatpho_http_started=""
  if declare -F __dybatpho_metrics_key > /dev/null; then
    __dybatpho_http_started="$(__dybatpho_log_now_ms)"
  fi
  while :; do
    local curl_args=(-fsSL -D "${header_file}" -w '%{http_code}' -o "${output}")
    [[ -n "${DYBATPHO_CURL_CONNECT_TIMEOUT}" ]] && curl_args+=(--connect-timeout "${DYBATPHO_CURL_CONNECT_TIMEOUT}")
    [[ -n "${DYBATPHO_CURL_TIMEOUT}" ]] && curl_args+=(--max-time "${DYBATPHO_CURL_TIMEOUT}")
    curl_args+=(${secret_args[@]+"${secret_args[@]}"})
    curl_args+=("$@")

    : > "${header_file}"
    if [[ "${body_on_stdin}" == true ]]; then
      code=$(command curl "${curl_args[@]}" "${url}" <<< "${DYBATPHO_CURL_SECRET_DATA}") || {
        code="000"
        dybatpho::error "Error when access ${url}"
      }
    else
      code=$(command curl "${curl_args[@]}" "${url}") || {
        code="000"
        dybatpho::error "Error when access ${url}"
      }
    fi

    local code_description
    code_description=$(__dybatpho_network_get_http_code "${code}")
    dybatpho::debug "Received HTTP status: ${code_description}"
    if [[ "${code}" =~ ^2[0-9][0-9]$ ]]; then
      rm -f "${header_file}" ${secret_config:+"${secret_config}"}
      break
    elif [[ "${code}" =~ ^4[0-9][0-9]$ ]]; then
      case "${code}" in
        408 | 425 | 429) ;; # kcov(skip)
        *)
          rm -f "${header_file}" ${secret_config:+"${secret_config}"}
          break
          ;;
      esac
    fi
    if ((attempt >= DYBATPHO_CURL_MAX_RETRIES)); then
      dybatpho::warn "No more retries left to run curl ${url}."
      rm -f "${header_file}" ${secret_config:+"${secret_config}"}
      break
    fi

    attempt=$((attempt + 1))
    delay=$((DYBATPHO_CURL_RETRY_BASE_DELAY * (2 ** (attempt - 1))))
    ((delay > DYBATPHO_CURL_RETRY_MAX_DELAY)) && delay="${DYBATPHO_CURL_RETRY_MAX_DELAY}"
    retry_after=$(awk 'tolower($1) == "retry-after:" { gsub("\r", "", $2); if ($2 ~ /^[0-9]+$/) print $2; exit }' "${header_file}")
    [[ -n "${retry_after}" ]] && delay="${retry_after}"
    ((delay > DYBATPHO_CURL_RETRY_MAX_DELAY)) && delay="${DYBATPHO_CURL_RETRY_MAX_DELAY}"
    if dybatpho::is true "${DYBATPHO_CURL_RETRY_JITTER}"; then
      ((delay += RANDOM % (DYBATPHO_CURL_RETRY_BASE_DELAY + 1)))
      ((delay > DYBATPHO_CURL_RETRY_MAX_DELAY)) && delay="${DYBATPHO_CURL_RETRY_MAX_DELAY}"
    fi
    if declare -F __dybatpho_metrics_key > /dev/null; then
      dybatpho::metrics_counter_inc dybatpho_http_retries_total
    fi
    dybatpho::progress "Retrying in ${delay} seconds (${attempt}/${DYBATPHO_CURL_MAX_RETRIES})..."
    sleep "${delay}" || true
  done

  # Record how long the request took, including every retry, so that the metric
  # reflects what the script actually waited for.
  if [[ -n "${__dybatpho_http_started}" ]]; then
    dybatpho::metrics_observe_ms dybatpho_http_request_duration_seconds \
      "$(($(__dybatpho_log_now_ms) - __dybatpho_http_started))" "status=${code}"
    dybatpho::metrics_counter_inc dybatpho_http_requests_total 1 "status=${code}"
  fi

  # Return exit code based on HTTP status code
  case "${code}" in
    '2'*) return 0 ;;
    '3'*) return 3 ;;
    '4'*) return 4 ;;
    '5'*) return 5 ;;
    *) return 1 ;;
  esac
}

#######################################
# @description Download file
# @arg $1 string URL
# @arg $2 string Destination of file to download
# @arg $@ string Other options/arguments for curl
# @see dybatpho::curl_do
# @exitcode 6 Can't create folder of destination file
# @tip The destination directory is created automatically before downloading
#######################################
function dybatpho::curl_download {
  local url dst_file
  dybatpho::expect_args url dst_file -- "$@"
  shift 2
  dybatpho::progress "Downloading ${url}"

  # Create destination folder
  local dst_dir
  dst_dir=$(dirname "${dst_file}") || return 6
  mkdir -p "${dst_dir}" || return 6

  dybatpho::curl_do "${url}" "${dst_file}" -# --no-silent "$@"
}

#######################################
# @description Transfer JSON data with URL by curl.
# @arg $1 string URL
# @arg $2 string Location of curl output, default is `/dev/null`
# @arg $@ string Other options/arguments for curl
# @see dybatpho::curl_do
# @exitcode 0 Transferred data
#######################################
function dybatpho::curl_json {
  local url
  dybatpho::expect_args url -- "$@"
  local output="/dev/null"
  shift
  if (($# > 0)); then
    output="$1"
    shift
  fi
  dybatpho::curl_do "${url}" "${output}" \
    --header "Accept: application/json" \
    --header "Content-Type: application/json" \
    "$@"
}

#######################################
# @description Fetch only HTTP headers for a URL by curl.
# @arg $1 string URL
# @arg $2 string Location of curl output, default is `/dev/null`
# @arg $@ string Other options/arguments for curl
# @see dybatpho::curl_do
# @exitcode 0 Transferred headers
#######################################
function dybatpho::curl_head {
  local url
  dybatpho::expect_args url -- "$@"
  local output="/dev/null"
  shift
  if (($# > 0)); then
    output="$1"
    shift
  fi
  dybatpho::curl_do "${url}" "${output}" -I "$@"
}

#######################################
# @description Upload fields and files with curl using multipart/form-data.
# @example
#   dybatpho::curl_upload https://example.com/upload /tmp/resp.json note="nightly run" report=@/tmp/report.csv
#   dybatpho::curl_upload https://example.com/upload /tmp/resp.json report=@/tmp/report.csv --request PUT
#
# @arg $1 string URL
# @arg $2 string Location of curl output, default is `/dev/null`
# @arg $@ string Form fields as `name=value` or `name=@path` pairs, plus any other curl options/arguments
# @see dybatpho::curl_do
# @exitcode 2 A `name=@path` field references a file that does not exist
# @tip The request method defaults to `POST`; pass `--request PUT` (or similar) afterwards to override it
#######################################
function dybatpho::curl_upload {
  local url
  dybatpho::expect_args url -- "$@"
  shift
  local output="/dev/null"
  if (($# > 0)); then
    output="$1"
    shift
  fi

  local curl_args=() field path
  while (($#)); do
    field="$1"
    shift
    if [[ "${field}" =~ ^[^=]+=@(.+)$ ]]; then
      path="${BASH_REMATCH[1]}"
      dybatpho::is file "${path}" || dybatpho::die "Upload file not found: ${path}" 2
      curl_args+=(-F "${field}")
    elif [[ "${field}" == *=* ]]; then
      curl_args+=(-F "${field}")
    else
      curl_args+=("${field}")
    fi
  done

  dybatpho::curl_do "${url}" "${output}" --request POST "${curl_args[@]}"
}

#######################################
# @description Verify a downloaded file against an expected checksum.
# @arg $1 string File to verify
# @arg $2 string Expected checksum as `algorithm:hexdigest`, algorithm is one of `sha256`, `sha1`, or `md5`
# @exitcode 7 Checksum mismatch
# @exitcode 8 Unsupported algorithm, invalid spec, or the checksum tool isn't installed
#######################################
function dybatpho::verify_checksum {
  local file checksum
  dybatpho::expect_args file checksum -- "$@"

  [[ "${checksum}" =~ ^(sha256|sha1|md5):([0-9a-fA-F]+)$ ]] \
    || dybatpho::die "Invalid checksum spec: ${checksum}" 8
  local algorithm="${BASH_REMATCH[1]}" expected="${BASH_REMATCH[2],,}"
  # `dybatpho::file_hash` already knows every spelling of this: `*sum` where it
  # exists, then `shasum`, `md5`, or `openssl`. Naming `sha256sum` directly here
  # meant this function died on a system that ships only `shasum` — macOS — while
  # the rest of the library coped.
  local actual
  actual=$(dybatpho::file_hash "${file}" "${algorithm}") \
    || dybatpho::die "Unable to compute ${algorithm} checksum for ${file}" 8
  if [[ "${actual,,}" != "${expected}" ]]; then
    dybatpho::error "Checksum mismatch for ${file}: expected ${expected}, got ${actual}"
    return 7
  fi
  dybatpho::debug "Checksum verified for ${file} (${algorithm})"
}

#######################################
# @description Download a file with resume support and optional checksum verification.
# @example
#   dybatpho::curl_resume_download https://example.com/big.iso /tmp/big.iso
#   dybatpho::curl_resume_download https://example.com/big.iso /tmp/big.iso sha256:3a7bd3e2360a3d...
#
# @arg $1 string URL
# @arg $2 string Destination of file to download
# @arg $3 string Optional checksum as `algorithm:hexdigest` (sha256, sha1, or md5)
# @arg $@ string Other options/arguments for curl
# @see dybatpho::curl_download
# @see dybatpho::verify_checksum
# @exitcode 6 Can't create folder of destination file
# @exitcode 7 Checksum verification failed
# @exitcode 8 Unsupported checksum algorithm or missing checksum tool
# @tip A partially downloaded destination file is resumed instead of restarted
#######################################
function dybatpho::curl_resume_download {
  local url dst_file
  dybatpho::expect_args url dst_file -- "$@"
  shift 2

  local checksum=""
  if (($# > 0)) && [[ "$1" =~ ^(sha256|sha1|md5):[0-9a-fA-F]+$ ]]; then
    checksum="$1"
    shift
  fi

  local dst_dir
  dst_dir=$(dirname "${dst_file}") || return 6
  mkdir -p "${dst_dir}" || return 6

  dybatpho::progress "Downloading ${url} (resume enabled)"
  dybatpho::curl_do "${url}" "${dst_file}" -# --no-silent -C - "$@" || return $?

  if [[ -n "${checksum}" ]]; then
    dybatpho::verify_checksum "${dst_file}" "${checksum}" || return $?
  fi
}

#######################################
# @description Parse a raw curl header dump (and optional body file) into normalized response state.
# @arg $1 string Path to a header file captured via `curl -D` (may contain multiple header blocks from redirects; the last block wins)
# @arg $2 string Optional path to the response body file to record
# @set DYBATPHO_HTTP_STATUS number Status code of the last received response block
# @set DYBATPHO_HTTP_HEADERS map Lower-cased header name to value, from the last response block
# @set DYBATPHO_HTTP_BODY_FILE string Path to the response body, or empty when omitted
# @exitcode 1 No status line was found in the header file
#######################################
function dybatpho::curl_parse_response {
  local header_file
  dybatpho::expect_args header_file -- "$@"
  shift
  local body_file="${1:-}"
  (($# > 0)) && shift

  dybatpho::is file "${header_file}" || dybatpho::die "Header file not found: ${header_file}"

  DYBATPHO_HTTP_STATUS=""
  DYBATPHO_HTTP_HEADERS=()
  # shellcheck disable=SC2034 # documented output variable, read by callers
  DYBATPHO_HTTP_BODY_FILE="${body_file}"

  local line key value
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    if [[ "${line}" =~ ^HTTP/[0-9.]+[[:space:]]+([0-9]{3}) ]]; then
      DYBATPHO_HTTP_STATUS="${BASH_REMATCH[1]}"
      DYBATPHO_HTTP_HEADERS=() # A new status line starts a new block, e.g. after a redirect.
      continue
    fi
    [[ -z "${line}" ]] && continue
    if [[ "${line}" =~ ^([^:]+):[[:space:]]?(.*)$ ]]; then
      key="${BASH_REMATCH[1],,}"
      value="${BASH_REMATCH[2]}"
      DYBATPHO_HTTP_HEADERS["${key}"]="${value}"
    fi
  done < "${header_file}"

  [[ -n "${DYBATPHO_HTTP_STATUS}" ]]
}

#######################################
# @description Print a normalized response header captured by `dybatpho::curl_parse_response`.
# @arg $1 string Header name, matched case-insensitively
# @arg $2 string Optional default value
# @stdout Header value
# @exitcode 1 Header is missing and no default was supplied
#######################################
function dybatpho::curl_response_header {
  local name
  dybatpho::expect_args name -- "$@"
  local name_lower="${name,,}"
  if [[ -v "DYBATPHO_HTTP_HEADERS[${name_lower}]" ]]; then
    printf '%s\n' "${DYBATPHO_HTTP_HEADERS[${name_lower}]}"
  elif (($# > 1)); then
    printf '%s\n' "$2"
  else
    return 1
  fi
}

#######################################
# @description Perform a request via `dybatpho::curl_do` and parse its response into normalized status/header/body state.
# @example
#   dybatpho::curl_request https://example.com/api /tmp/resp.json
#   echo "${DYBATPHO_HTTP_STATUS}"
#   dybatpho::curl_response_header content-type
#
# @arg $1 string URL
# @arg $2 string Location of curl output, default is `/dev/null`
# @arg $@ string Other options/arguments for curl
# @see dybatpho::curl_do
# @see dybatpho::curl_parse_response
# @set DYBATPHO_HTTP_STATUS number Status code of the last received response block
# @set DYBATPHO_HTTP_HEADERS map Lower-cased header name to value, from the last response block
# @set DYBATPHO_HTTP_BODY_FILE string Path holding the response body
# @tip Response headers aren't captured while `DRY_RUN` is enabled
#######################################
function dybatpho::curl_request {
  local url
  dybatpho::expect_args url -- "$@"
  shift
  local output="/dev/null"
  if (($# > 0)); then
    output="$1"
    shift
  fi

  local header_file
  dybatpho::create_temp header_file ".headers"
  local exit_code=0
  dybatpho::curl_do "${url}" "${output}" "$@" -D "${header_file}" || exit_code=$?

  if ! dybatpho::is true "${DRY_RUN}"; then
    dybatpho::curl_parse_response "${header_file}" "${output}"
  fi
  return "${exit_code}"
}

#######################################
# @description Perform a request via `dybatpho::curl_do` with connect/total timeouts scoped to this call only.
# @example
#   dybatpho::curl_timeout https://example.com /tmp/out 2 10
#   dybatpho::curl_timeout https://example.com /tmp/out "" 5 --header "X-Test: 1"
#
# @arg $1 string URL
# @arg $2 string Location of curl output, default is `/dev/null`
# @arg $3 number Connect timeout in seconds for this request, empty keeps the global default
# @arg $4 number Total timeout in seconds for this request, empty keeps the global default
# @arg $@ string Other options/arguments for curl
# @see dybatpho::curl_do
# @tip Overrides apply only for the duration of this call; global `DYBATPHO_CURL_*` timeouts are unaffected
#######################################
function dybatpho::curl_timeout {
  local url
  dybatpho::expect_args url -- "$@"
  shift
  local output="/dev/null"
  if (($# > 0)); then
    output="$1"
    shift
  fi

  local connect_timeout="" total_timeout=""
  if (($# > 0)); then
    connect_timeout="$1"
    shift
  fi
  if (($# > 0)); then
    total_timeout="$1"
    shift
  fi
  [[ -z "${connect_timeout}" || "${connect_timeout}" =~ ^[0-9]+$ ]] \
    || dybatpho::die "Connect timeout must be a non-negative integer"
  [[ -z "${total_timeout}" || "${total_timeout}" =~ ^[0-9]+$ ]] \
    || dybatpho::die "Total timeout must be a non-negative integer"

  (
    [[ -n "${connect_timeout}" ]] && DYBATPHO_CURL_CONNECT_TIMEOUT="${connect_timeout}"
    [[ -n "${total_timeout}" ]] && DYBATPHO_CURL_TIMEOUT="${total_timeout}"
    dybatpho::curl_do "${url}" "${output}" "$@"
  )
}

#######################################
# @description Report whether a circuit breaker key is currently open, half-open, or closed.
# @arg $1 string Circuit key
# @stdout `open`, `half-open`, or `closed`
#######################################
function dybatpho::circuit_state {
  local key
  dybatpho::expect_args key -- "$@"
  local opened_at="${DYBATPHO_CIRCUIT_OPENED_AT[${key}]:-0}"
  if ((opened_at == 0)); then
    echo closed
    return 0
  fi
  local now elapsed
  now=$(date +%s)
  elapsed=$((now - opened_at))
  if ((elapsed < DYBATPHO_CIRCUIT_COOLDOWN)); then
    echo open
  else
    echo half-open
  fi
}

#######################################
# @description Reset a circuit breaker key back to the closed state.
# @arg $1 string Circuit key
#######################################
function dybatpho::circuit_reset {
  local key
  dybatpho::expect_args key -- "$@"
  DYBATPHO_CIRCUIT_FAILURES["${key}"]=0
  DYBATPHO_CIRCUIT_OPENED_AT["${key}"]=0
}

#######################################
# @description Run a shell command guarded by a circuit breaker keyed by name.
# @example
#   dybatpho::circuit_breaker api.example.com "dybatpho::curl_do https://api.example.com/health"
#
# @arg $1 string Circuit key, typically a host or service name
# @arg $2 string Shell command string to run
# @env DYBATPHO_CIRCUIT_THRESHOLD number Consecutive failures before the circuit opens (default `5`)
# @env DYBATPHO_CIRCUIT_COOLDOWN number Seconds the circuit stays open before a trial request is allowed (default `30`)
# @exitcode 0 The command succeeded, or a trial request succeeded and closed the circuit
# @exitcode 9 The circuit is open; the command was not attempted
# @exitcode other The command's own exit code, while the circuit is closed or half-open
# @tip The command is executed with `eval`, so pass it as one shell command string
# @note Circuit breaker state is in-memory and process-local; it does not persist across script invocations
#######################################
function dybatpho::circuit_breaker {
  local key command
  dybatpho::expect_args key command -- "$@"
  shift 2

  local failures="${DYBATPHO_CIRCUIT_FAILURES[${key}]:-0}"
  local opened_at="${DYBATPHO_CIRCUIT_OPENED_AT[${key}]:-0}"
  local now
  now=$(date +%s)

  if ((opened_at > 0)); then
    local elapsed=$((now - opened_at))
    if ((elapsed < DYBATPHO_CIRCUIT_COOLDOWN)); then
      dybatpho::warn "Circuit '${key}' is open; skipping request (retry in $((DYBATPHO_CIRCUIT_COOLDOWN - elapsed))s)"
      return 9
    fi
    dybatpho::debug "Circuit '${key}' cooldown elapsed; allowing a trial request"
  fi

  local exit_code=0
  eval "${command}" || exit_code=$?

  if ((exit_code == 0)); then
    DYBATPHO_CIRCUIT_FAILURES["${key}"]=0
    DYBATPHO_CIRCUIT_OPENED_AT["${key}"]=0
  else
    failures=$((failures + 1))
    DYBATPHO_CIRCUIT_FAILURES["${key}"]="${failures}"
    if ((failures >= DYBATPHO_CIRCUIT_THRESHOLD)); then
      DYBATPHO_CIRCUIT_OPENED_AT["${key}"]="${now}"
      dybatpho::warn "Circuit '${key}' opened after ${failures} consecutive failures"
    fi
  fi
  return "${exit_code}"
}

#######################################
# @description Split the authority of a URL into user, password, host, and port.
#   The authority is the awkward part of the grammar: everything in it is
#   optional, the delimiters repeat, and an IPv6 literal carries colons of its
#   own inside brackets.
# @arg $1 string Authority, such as `user:pass@host:443` or `[::1]:8080`
# @set DYBATPHO_URL The `user`, `password`, `host`, and `port` entries
# @exitcode 1 The authority names no host, or a port that is not a port
#######################################
function __dybatpho_network_parse_authority {
  local authority="$1"
  local userinfo="" hostport="${authority}"
  # The last `@` separates the credentials, so that a password containing one
  # does not move the boundary.
  if [[ "${authority}" == *@* ]]; then
    userinfo="${authority%@*}"
    hostport="${authority##*@}"
  fi

  if [[ -n "${userinfo}" ]]; then
    if [[ "${userinfo}" == *:* ]]; then
      DYBATPHO_URL[user]="${userinfo%%:*}"
      DYBATPHO_URL[password]="${userinfo#*:}"
    else
      DYBATPHO_URL[user]="${userinfo}"
    fi
  fi

  local host="" port=""
  if [[ "${hostport}" == \[*\]* ]]; then
    # An IPv6 literal is bracketed precisely so its colons cannot be read as a
    # port separator.
    host="${hostport%%\]*}"
    host="${host#\[}"
    local after="${hostport#*\]}"
    if [[ -n "${after}" ]]; then
      [[ "${after}" == :* ]] || return 1
      port="${after#:}"
    fi
  elif [[ "${hostport}" == *:* ]]; then
    host="${hostport%%:*}"
    port="${hostport#*:}"
    # A bare host may not contain a colon, so anything left is not a port.
    [[ "${port}" != *:* ]] || return 1
  else
    host="${hostport}"
  fi

  [[ -n "${host}" ]] || return 1
  if [[ -n "${port}" ]]; then
    __dybatpho_network_is_port "${port}" || return 1
  fi

  DYBATPHO_URL[host]="${host}"
  DYBATPHO_URL[port]="${port}"
}

#######################################
# @description Return success when a value is a usable TCP or UDP port number.
# @arg $1 string Value to test
# @exitcode 0 The value is a decimal number from 1 to 65535
# @exitcode 1 It is not
#######################################
function __dybatpho_network_is_port {
  [[ "$1" =~ ^[0-9]{1,5}$ ]] || return 1
  ((10#$1 >= 1 && 10#$1 <= 65535))
}

#######################################
# @description Split a URL into its components.
#   The result lands in `DYBATPHO_URL`, one entry per component, the way
#   `dybatpho::curl_parse_response` leaves a response in `DYBATPHO_HTTP_*`.
#   Every entry is always present; a component the URL omits is empty, so a
#   caller reads it without guarding against an unset key.
#
#   A scheme and `://` are required. `mailto:someone@example.com` has neither an
#   authority nor a host, and guessing what its parts are called would be
#   inventing an answer rather than parsing one.
#
#   The components are returned exactly as written. Percent-escapes are left
#   alone, because decoding them here would destroy the difference between a
#   separator and a character that merely looks like one; `dybatpho::url_decode`
#   is there for the caller that wants it.
# @example
#   dybatpho::url_parse "https://user:secret@example.com:8443/a/b?q=1#top"
#   printf '%s\n' "${DYBATPHO_URL[host]}"    # example.com
#   printf '%s\n' "${DYBATPHO_URL[port]}"    # 8443
#   printf '%s\n' "${DYBATPHO_URL[path]}"    # /a/b
#
# @example
#   dybatpho::url_parse "http://[::1]:8080/health"
#   printf '%s\n' "${DYBATPHO_URL[host]}"    # ::1
#
# @arg $1 string URL to split
# @set DYBATPHO_URL map The `scheme`, `user`, `password`, `host`, `port`, `path`, `query`, and `fragment` of the URL
# @exitcode 0 The URL was split
# @exitcode 1 The URL has no scheme, no host, or a port that is not a port
# @see
#   - `dybatpho::url_part`
#   - `dybatpho::url_decode`
#######################################
function dybatpho::url_parse {
  local url
  dybatpho::expect_args url -- "$@"
  DYBATPHO_URL=(
    [scheme]="" [user]="" [password]="" [host]=""
    [port]="" [path]="" [query]="" [fragment]=""
  )
  [[ "${url}" =~ ${__DYBATPHO_URL_REGEX} ]] || return 1

  DYBATPHO_URL[scheme]="${BASH_REMATCH[1],,}"
  DYBATPHO_URL[path]="${BASH_REMATCH[3]}"
  DYBATPHO_URL[query]="${BASH_REMATCH[5]}"
  DYBATPHO_URL[fragment]="${BASH_REMATCH[7]}"
  __dybatpho_network_parse_authority "${BASH_REMATCH[2]}" || return 1
}

#######################################
# @description Print one component of the last parsed URL.
# @example
#   dybatpho::url_parse "https://example.com/health"
#   dybatpho::url_part host            # example.com
#   dybatpho::url_part port 443        # 443, the default, since none was given
#
# @arg $1 string Component name: `scheme`, `user`, `password`, `host`, `port`, `path`, `query`, or `fragment`
# @arg $2 string Optional value to print when the component is empty
# @stdout The component, or the default
# @exitcode 1 The component is empty and no default was supplied
# @exitcode 1 Stop the script when the name is not a component of a URL
# @see
#   - `dybatpho::url_parse`
#######################################
function dybatpho::url_part {
  local name
  dybatpho::expect_args name -- "$@"
  [[ -v "DYBATPHO_URL[${name}]" ]] \
    || dybatpho::die "${FUNCNAME[0]}: '${name}' is not a component of a URL"
  if [[ -n "${DYBATPHO_URL[${name}]}" ]]; then
    printf '%s\n' "${DYBATPHO_URL[${name}]}"
  elif (($# > 1)); then
    printf '%s\n' "$2"
  else
    return 1
  fi
}

#######################################
# @description Split an IPv4 address into its four octets as numbers.
#   A leading zero is rejected rather than ignored. `inet_aton` and much of the
#   software built on it read `010` as octal, so `127.0.0.010` is one host to
#   one parser and another host to the next. An address that means two things
#   is not an address this library will agree to.
# @arg $1 string Address to split
# @arg $2 string Name of the array variable receiving the four octets
# @set The named array, to four numbers from 0 to 255
# @exitcode 1 The value is not an IPv4 address
#######################################
function __dybatpho_network_ipv4_octets {
  local __ipv4_address="$1"
  local -n __ipv4_out="$2"
  [[ "${__ipv4_address}" =~ ^([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})$ ]] \
    || return 1
  local -a __ipv4_matched=("${BASH_REMATCH[@]:1:4}")
  local __ipv4_octet
  __ipv4_out=()
  for __ipv4_octet in "${__ipv4_matched[@]}"; do
    [[ "${__ipv4_octet}" == "0" || "${__ipv4_octet}" != 0* ]] || return 1
    ((10#${__ipv4_octet} <= 255)) || return 1
    __ipv4_out+=("$((10#${__ipv4_octet}))")
  done
}

#######################################
# @description Expand an IPv6 address into its eight groups as numbers.
#   Everything an IPv6 address may leave out is put back here: the `::` that
#   stands for a run of zero groups, and the dotted IPv4 tail that occupies the
#   last two groups of a mapped address. Comparing addresses is only simple once
#   both are written out in full.
#
#   A zone index such as `%eth0` is rejected. It names an interface rather than
#   a part of the address, and it is not comparable between two hosts.
# @arg $1 string Address to expand
# @arg $2 string Name of the array variable receiving the eight groups
# @set The named array, to eight numbers from 0 to 65535
# @exitcode 1 The value is not an IPv6 address
#######################################
function __dybatpho_network_ipv6_groups {
  local __ipv6_address="$1"
  local -n __ipv6_out="$2"
  [[ "${__ipv6_address}" != *%* ]] || return 1
  [[ "${__ipv6_address}" == *:* ]] || return 1
  # A single colon at either end belongs to a `::` or to nothing at all. Without
  # this, `read -a` drops the empty trailing field and `1:2:3:4:5:6:7:8:` would
  # count as eight groups.
  [[ "${__ipv6_address}" != *: || "${__ipv6_address}" == *:: ]] || return 1
  [[ "${__ipv6_address}" != :* || "${__ipv6_address}" == ::* ]] || return 1

  local __ipv6_head __ipv6_tail __ipv6_has_double=0
  if [[ "${__ipv6_address}" == *::* ]]; then
    __ipv6_has_double=1
    __ipv6_head="${__ipv6_address%%::*}"
    __ipv6_tail="${__ipv6_address#*::}"
    # `::` stands for "the rest is zero", so a second one has nothing left to say.
    [[ "${__ipv6_tail}" != *::* ]] || return 1
  else
    __ipv6_head="${__ipv6_address}"
    __ipv6_tail=""
  fi

  local -a __ipv6_head_parts=() __ipv6_tail_parts=()
  [[ -z "${__ipv6_head}" ]] || IFS=':' read -r -a __ipv6_head_parts <<< "${__ipv6_head}"
  [[ -z "${__ipv6_tail}" ]] || IFS=':' read -r -a __ipv6_tail_parts <<< "${__ipv6_tail}"

  # A dotted tail, as in `::ffff:192.0.2.1`, is two groups written in decimal.
  local -a __ipv6_mapped=()
  local __ipv6_mapped_in_tail=0 __ipv6_last=""
  if ((${#__ipv6_tail_parts[@]})); then
    __ipv6_last="${__ipv6_tail_parts[-1]}"
    __ipv6_mapped_in_tail=1
  elif ((${#__ipv6_head_parts[@]})); then
    __ipv6_last="${__ipv6_head_parts[-1]}"
  fi
  if [[ "${__ipv6_last}" == *.* ]]; then
    local -a __ipv6_octets=()
    __dybatpho_network_ipv4_octets "${__ipv6_last}" __ipv6_octets || return 1
    __ipv6_mapped=(
      "$(((__ipv6_octets[0] << 8) | __ipv6_octets[1]))"
      "$(((__ipv6_octets[2] << 8) | __ipv6_octets[3]))"
    )
    if ((__ipv6_mapped_in_tail)); then
      unset '__ipv6_tail_parts[-1]'
    else
      unset '__ipv6_head_parts[-1]'
    fi
  else
    __ipv6_mapped_in_tail=0
  fi

  local -a __ipv6_lead=() __ipv6_trail=()
  local __ipv6_part
  for __ipv6_part in ${__ipv6_head_parts[@]+"${__ipv6_head_parts[@]}"}; do
    [[ "${__ipv6_part}" =~ ${__DYBATPHO_IPV6_GROUP_REGEX} ]] || return 1
    __ipv6_lead+=("$((16#${__ipv6_part}))")
  done
  for __ipv6_part in ${__ipv6_tail_parts[@]+"${__ipv6_tail_parts[@]}"}; do
    [[ "${__ipv6_part}" =~ ${__DYBATPHO_IPV6_GROUP_REGEX} ]] || return 1
    __ipv6_trail+=("$((16#${__ipv6_part}))")
  done
  if ((${#__ipv6_mapped[@]})); then
    if ((__ipv6_mapped_in_tail)); then
      __ipv6_trail+=("${__ipv6_mapped[@]}")
    else
      __ipv6_lead+=("${__ipv6_mapped[@]}")
    fi
  fi

  local __ipv6_have=$((${#__ipv6_lead[@]} + ${#__ipv6_trail[@]}))
  local __ipv6_index
  if ((__ipv6_has_double)); then
    # `::` has to stand for at least one group, or it would be spelled `:`.
    ((__ipv6_have <= 7)) || return 1
    for ((__ipv6_index = __ipv6_have; __ipv6_index < 8; __ipv6_index++)); do
      __ipv6_lead+=(0)
    done
  else
    ((__ipv6_have == 8)) || return 1
  fi

  __ipv6_out=("${__ipv6_lead[@]}" ${__ipv6_trail[@]+"${__ipv6_trail[@]}"})
}

#######################################
# @description Return success when a value is an IPv4 address.
# @example
#   dybatpho::is_ipv4 192.0.2.10      # yes
#   dybatpho::is_ipv4 192.0.2.256     # no
#   dybatpho::is_ipv4 127.0.0.010     # no, a leading zero is ambiguous
#
# @arg $1 string Value to test
# @exitcode 0 The value is an IPv4 address
# @exitcode 1 It is not
#######################################
function dybatpho::is_ipv4 {
  local address
  dybatpho::expect_args address -- "$@"
  # shellcheck disable=SC2034 # octets is filled through a nameref; only the exit code is wanted
  local -a octets=()
  __dybatpho_network_ipv4_octets "${address}" octets
}

#######################################
# @description Return success when a value is an IPv6 address.
# @example
#   dybatpho::is_ipv6 ::1                    # yes
#   dybatpho::is_ipv6 2001:db8::1            # yes
#   dybatpho::is_ipv6 ::ffff:192.0.2.1       # yes
#   dybatpho::is_ipv6 2001:db8::1::2         # no, one `::` is all there is
#
# @arg $1 string Value to test
# @exitcode 0 The value is an IPv6 address
# @exitcode 1 It is not
# @note A zone index such as `fe80::1%eth0` is refused: it names an interface
#   rather than a part of the address
#######################################
function dybatpho::is_ipv6 {
  local address
  dybatpho::expect_args address -- "$@"
  # shellcheck disable=SC2034 # groups is filled through a nameref; only the exit code is wanted
  local -a groups=()
  __dybatpho_network_ipv6_groups "${address}" groups
}

#######################################
# @description Print which version of IP an address is.
# @example
#   dybatpho::ip_version 192.0.2.10    # 4
#   dybatpho::ip_version ::1           # 6
#
# @arg $1 string Address to inspect
# @stdout `4` or `6`
# @exitcode 1 The value is not an IP address of either version
#######################################
function dybatpho::ip_version {
  local address
  dybatpho::expect_args address -- "$@"
  if dybatpho::is_ipv4 "${address}"; then
    printf '4\n'
  elif dybatpho::is_ipv6 "${address}"; then
    printf '6\n'
  else
    return 1
  fi
}

#######################################
# @description Split a CIDR block into its address and prefix length.
# @arg $1 string Block such as `10.0.0.0/8` or `2001:db8::/32`
# @arg $2 string Name of the variable receiving the address
# @arg $3 string Name of the variable receiving the prefix length
# @arg $4 string Name of the variable receiving the IP version
# @set The three named variables
# @exitcode 1 The value is not a CIDR block
#######################################
function __dybatpho_network_parse_cidr {
  local __cidr_block="$1"
  local -n __cidr_address_out="$2"
  local -n __cidr_prefix_out="$3"
  local -n __cidr_version_out="$4"
  [[ "${__cidr_block}" == */* ]] || return 1
  local __cidr_address="${__cidr_block%/*}"
  local __cidr_prefix="${__cidr_block##*/}"
  [[ "${__cidr_prefix}" =~ ^[0-9]{1,3}$ ]] || return 1
  # A leading zero here is the same ambiguity as in an octet, and `/08` is not a
  # prefix length anyone writes on purpose.
  [[ "${__cidr_prefix}" == "0" || "${__cidr_prefix}" != 0* ]] || return 1

  local __cidr_version
  __cidr_version="$(dybatpho::ip_version "${__cidr_address}")" || return 1
  if ((__cidr_version == 4)); then
    ((10#${__cidr_prefix} <= 32)) || return 1
  else
    ((10#${__cidr_prefix} <= 128)) || return 1
  fi

  __cidr_address_out="${__cidr_address}"
  __cidr_prefix_out="$((10#${__cidr_prefix}))"
  __cidr_version_out="${__cidr_version}"
}

#######################################
# @description Return success when a value is a CIDR block.
# @example
#   dybatpho::is_cidr 10.0.0.0/8        # yes
#   dybatpho::is_cidr 2001:db8::/32     # yes
#   dybatpho::is_cidr 10.0.0.0/33       # no
#
# @arg $1 string Value to test
# @exitcode 0 The value is a CIDR block of either IP version
# @exitcode 1 It is not
#######################################
function dybatpho::is_cidr {
  local block
  dybatpho::expect_args block -- "$@"
  local address="" prefix="" version=""
  __dybatpho_network_parse_cidr "${block}" address prefix version
}

#######################################
# @description Print the dotted-decimal subnet mask of an IPv4 prefix length.
#   There is no dotted form of an IPv6 prefix, so this is IPv4 only: the
#   notation itself does not exist for the other version rather than being
#   left out here.
# @example
#   dybatpho::cidr_netmask 24    # 255.255.255.0
#   dybatpho::cidr_netmask 0     # 0.0.0.0
#
# @arg $1 string Prefix length from 0 to 32, with or without a leading `/`
# @stdout The subnet mask
# @exitcode 1 Stop the script when the value is not an IPv4 prefix length
#######################################
function dybatpho::cidr_netmask {
  local prefix
  dybatpho::expect_args prefix -- "$@"
  prefix="${prefix#/}"
  if ! [[ "${prefix}" =~ ^[0-9]{1,2}$ ]] || ((10#${prefix} > 32)); then
    dybatpho::die "${FUNCNAME[0]}: '${prefix}' is not an IPv4 prefix length"
  fi
  prefix="$((10#${prefix}))"
  local mask=0
  ((prefix == 0)) || mask=$(((0xFFFFFFFF << (32 - prefix)) & 0xFFFFFFFF))
  printf '%d.%d.%d.%d\n' \
    "$(((mask >> 24) & 0xFF))" "$(((mask >> 16) & 0xFF))" \
    "$(((mask >> 8) & 0xFF))" "$((mask & 0xFF))"
}

#######################################
# @description Return success when an address falls inside a CIDR block.
#   Both versions are supported, and an address is never inside a block of the
#   other version: `::ffff:10.0.0.1` and `10.0.0.1` name the same host to some
#   software, but they are not the same address and this does not pretend
#   otherwise.
# @example
#   dybatpho::cidr_contains 10.0.0.0/8 10.1.2.3            # yes
#   dybatpho::cidr_contains 10.0.0.0/8 11.1.2.3            # no
#   dybatpho::cidr_contains 2001:db8::/32 2001:db8::1      # yes
#   dybatpho::cidr_contains 0.0.0.0/0 203.0.113.1          # yes
#
# @arg $1 string CIDR block
# @arg $2 string Address to test
# @exitcode 0 The address is inside the block
# @exitcode 1 It is not
# @exitcode 1 Stop the script when the block or the address is malformed
#######################################
function dybatpho::cidr_contains {
  local block address
  dybatpho::expect_args block address -- "$@"
  local network="" prefix="" version=""
  __dybatpho_network_parse_cidr "${block}" network prefix version \
    || dybatpho::die "${FUNCNAME[0]}: '${block}' is not a CIDR block"
  local address_version
  address_version="$(dybatpho::ip_version "${address}")" \
    || dybatpho::die "${FUNCNAME[0]}: '${address}' is not an IP address"
  ((address_version == version)) || return 1

  if ((version == 4)); then
    local -a network_octets=() address_octets=()
    __dybatpho_network_ipv4_octets "${network}" network_octets
    __dybatpho_network_ipv4_octets "${address}" address_octets
    local network_int=$(((network_octets[0] << 24) | (network_octets[1] << 16) | (network_octets[2] << 8) | network_octets[3]))
    local address_int=$(((address_octets[0] << 24) | (address_octets[1] << 16) | (address_octets[2] << 8) | address_octets[3]))
    local mask=0
    ((prefix == 0)) || mask=$(((0xFFFFFFFF << (32 - prefix)) & 0xFFFFFFFF))
    (((network_int & mask) == (address_int & mask)))
    return
  fi

  local -a network_groups=() address_groups=()
  __dybatpho_network_ipv6_groups "${network}" network_groups
  __dybatpho_network_ipv6_groups "${address}" address_groups
  local remaining="${prefix}" index bits mask
  for ((index = 0; index < 8; index++)); do
    ((remaining > 0)) || break
    bits=$((remaining >= 16 ? 16 : remaining))
    mask=$(((0xFFFF << (16 - bits)) & 0xFFFF))
    (((network_groups[index] & mask) == (address_groups[index] & mask))) || return 1
    remaining=$((remaining - bits))
  done
  return 0
}

#######################################
# @description Return success when a TCP port accepts a connection.
#   The connection is made with Bash's own `/dev/tcp`, so nothing has to be
#   installed for this to work. A build of Bash compiled without network
#   redirections cannot do it, and reports the port as closed.
#
#   The host and the port are passed to the timed-out shell as arguments rather
#   than spliced into the script it runs, so a host name is never read as code.
# @example
#   dybatpho::port_open localhost 5432
#   dybatpho::port_open db.internal 5432 2
#
# @arg $1 string Host name or address
# @arg $2 number Port
# @arg $3 number Seconds to wait, defaulting to `DYBATPHO_PORT_TIMEOUT`
# @env DYBATPHO_PORT_TIMEOUT number Seconds to wait for the connection
# @exitcode 0 The port accepted a connection
# @exitcode 1 It did not, within the timeout
# @exitcode 1 Stop the script when the port or the timeout is not a number
# @note The timeout needs the `timeout` command; without it the connection waits
#   as long as the system's own TCP timeout
# @see
#   - `dybatpho::wait_port`
#######################################
function dybatpho::port_open {
  local host port
  dybatpho::expect_args host port -- "$@"
  __dybatpho_network_is_port "${port}" \
    || dybatpho::die "${FUNCNAME[0]}: '${port}' is not a port number"
  local seconds="${3:-${DYBATPHO_PORT_TIMEOUT}}"
  [[ "${seconds}" =~ ^[0-9]+$ ]] \
    || dybatpho::die "${FUNCNAME[0]}: '${seconds}' is not a number of seconds"

  if dybatpho::is command timeout; then
    # shellcheck disable=SC2016 # `$1` and `$2` are the inner shell's arguments, and
    # keeping them unexpanded here is the point: the host never becomes code.
    timeout "${seconds}" "${BASH}" -c 'exec 3<>/dev/tcp/"$1"/"$2"' \
      dybatpho-port-open "${host}" "${port}" 2> /dev/null
    return
  fi
  (exec 3<> "/dev/tcp/${host}/${port}") 2> /dev/null
}

#######################################
# @description Wait until a TCP port accepts a connection.
#   This is the wait a script does after starting a service and before using it,
#   written once. Each attempt is given no more time than the wait has left, so
#   the whole call keeps to its budget rather than overrunning it by the length
#   of one connection attempt.
# @example
#   docker compose up -d
#   dybatpho::wait_port localhost 5432 60 \
#     || dybatpho::die "The database never came up"
#
# @arg $1 string Host name or address
# @arg $2 number Port
# @arg $3 number Seconds to keep trying, defaulting to `DYBATPHO_WAIT_PORT_TIMEOUT`
# @arg $4 number Seconds between attempts, defaulting to `DYBATPHO_WAIT_PORT_INTERVAL`
# @env DYBATPHO_WAIT_PORT_TIMEOUT number Seconds to keep trying
# @env DYBATPHO_WAIT_PORT_INTERVAL number Seconds between attempts
# @exitcode 0 The port accepted a connection before the time ran out
# @exitcode 1 It never did
# @exitcode 1 Stop the script when an argument is not a number
# @see
#   - `dybatpho::port_open`
#######################################
function dybatpho::wait_port {
  local host port
  dybatpho::expect_args host port -- "$@"
  local total="${3:-${DYBATPHO_WAIT_PORT_TIMEOUT}}"
  local interval="${4:-${DYBATPHO_WAIT_PORT_INTERVAL}}"
  [[ "${total}" =~ ^[0-9]+$ ]] \
    || dybatpho::die "${FUNCNAME[0]}: '${total}' is not a number of seconds"
  [[ "${interval}" =~ ^[0-9]+$ ]] \
    || dybatpho::die "${FUNCNAME[0]}: '${interval}' is not a number of seconds"

  local deadline=$((SECONDS + total)) remaining attempt
  while :; do
    remaining=$((deadline - SECONDS))
    ((remaining > 0)) || remaining=1
    attempt=$((remaining < DYBATPHO_PORT_TIMEOUT ? remaining : DYBATPHO_PORT_TIMEOUT))
    dybatpho::port_open "${host}" "${port}" "${attempt}" && return 0
    ((SECONDS < deadline)) || return 1
    sleep "${interval}"
  done
}
