# network.sh

Utilities for network

> 🧭 Source: [src/network.sh](../src/network.sh)
>
> Jump to: [Overview](#overview) · [Tips](#tips) · [Reference](#reference)

<a id="overview"></a>
## ✨ Overview

This module contains functions to work with network connection, downloads,
JSON-oriented requests, and HEAD requests. It also provides multipart
uploads, resumable downloads with checksum verification, normalized
response parsing (status/headers/body), per-request timeouts, and an
in-memory circuit breaker.


Alongside the HTTP client it carries the primitives a script reaches for
before making a request at all: splitting a URL into its parts, deciding
whether a string is an address or a network, whether an address is inside
one, and whether a port is accepting connections yet. These are the checks
that otherwise get written inline as a regex that nearly works — the kind
that accepts `192.0.2.256`, or reads `127.0.0.010` as a different host than
the resolver does.

### 🌍 Environment

| Variable | Type | Description |
| --- | --- | --- |
| **`DYBATPHO_CURL_MAX_RETRIES`** | number | Max number of retry attempts when `dybatpho::curl_do` retries a request |
| **`DYBATPHO_CURL_RETRY_BASE_DELAY`** | number | Initial retry delay in seconds (default `2`) |
| **`DYBATPHO_CURL_RETRY_MAX_DELAY`** | number | Maximum retry delay in seconds (default `30`) |
| **`DYBATPHO_CURL_RETRY_JITTER`** | bool | Add up to one base delay of random jitter |
| **`DYBATPHO_CURL_CONNECT_TIMEOUT`** | number | Optional curl connection timeout in seconds |
| **`DYBATPHO_CURL_TIMEOUT`** | number | Optional curl total timeout in seconds |
| **`DYBATPHO_CIRCUIT_THRESHOLD`** | number | Consecutive failures before `dybatpho::circuit_breaker` opens a circuit (default `5`) |
| **`DYBATPHO_CIRCUIT_COOLDOWN`** | number | Seconds an open circuit waits before allowing a trial request (default `30`) |
| **`DYBATPHO_PORT_TIMEOUT`** | number | Seconds `dybatpho::port_open` waits for a connection (default `5`) |
| **`DYBATPHO_WAIT_PORT_TIMEOUT`** | number | Seconds `dybatpho::wait_port` keeps trying before giving up (default `30`) |
| **`DYBATPHO_WAIT_PORT_INTERVAL`** | number | Seconds `dybatpho::wait_port` sleeps between attempts (default `1`) |
| **`DYBATPHO_CURL_SECRET_HEADERS`** | array | Headers to pass out of band, as `Name: value` |
| **`DYBATPHO_CURL_SECRET_DATA`** | string | Request body to pass on stdin instead of in an argument |

### 🚀 Highlights

- [`__dybatpho_network_config_escape`](#__dybatpho_network_config_escape) — Escape a value for a double-quoted `curl` config parameter. `curl` reads a config file as `name = "value"`, where the value takes backslash escapes, so a backslash or a quote inside a header has to be escaped or it ends the value early.
- [`__dybatpho_network_secret_config`](#__dybatpho_network_secret_config) — Write the secret headers of the current request into a private config file for `curl --config`. The file is created under `umask 077` before anything is written to it, so the credential is never on disk in a mode another account could read, and it is removed as soon as the request is over. The path is an argument, which is public; the contents are not.
- [`__dybatpho_network_get_http_code`](#__dybatpho_network_get_http_code) — Get description of HTTP status code
- [`dybatpho::curl_do`](#dybatphocurl_do) — Transferring data with URL by curl
- [`dybatpho::curl_download`](#dybatphocurl_download) — Download file
- [`dybatpho::curl_json`](#dybatphocurl_json) — Transfer JSON data with URL by curl.
- [`dybatpho::curl_head`](#dybatphocurl_head) — Fetch only HTTP headers for a URL by curl.
- [`dybatpho::curl_upload`](#dybatphocurl_upload) — Upload fields and files with curl using multipart/form-data.
- [`dybatpho::verify_checksum`](#dybatphoverify_checksum) — Verify a downloaded file against an expected checksum.
- [`dybatpho::curl_resume_download`](#dybatphocurl_resume_download) — Download a file with resume support and optional checksum verification.
- [`dybatpho::curl_parse_response`](#dybatphocurl_parse_response) — Parse a raw curl header dump (and optional body file) into normalized response state.
- [`dybatpho::curl_response_header`](#dybatphocurl_response_header) — Print a normalized response header captured by `dybatpho::curl_parse_response`.
- [`dybatpho::curl_request`](#dybatphocurl_request) — Perform a request via `dybatpho::curl_do` and parse its response into normalized status/header/body state.
- [`dybatpho::curl_timeout`](#dybatphocurl_timeout) — Perform a request via `dybatpho::curl_do` with connect/total timeouts scoped to this call only.
- [`dybatpho::circuit_state`](#dybatphocircuit_state) — Report whether a circuit breaker key is currently open, half-open, or closed.
- [`dybatpho::circuit_reset`](#dybatphocircuit_reset) — Reset a circuit breaker key back to the closed state.
- [`dybatpho::circuit_breaker`](#dybatphocircuit_breaker) — Run a shell command guarded by a circuit breaker keyed by name.
- [`__dybatpho_network_parse_authority`](#__dybatpho_network_parse_authority) — Split the authority of a URL into user, password, host, and port. The authority is the awkward part of the grammar: everything in it is optional, the delimiters repeat, and an IPv6 literal carries colons of its own inside brackets.
- [`__dybatpho_network_is_port`](#__dybatpho_network_is_port) — Return success when a value is a usable TCP or UDP port number.
- [`dybatpho::url_parse`](#dybatphourl_parse) — Split a URL into its components. The result lands in `DYBATPHO_URL`, one entry per component, the way `dybatpho::curl_parse_response` leaves a response in `DYBATPHO_HTTP_*`. Every entry is always present; a component the URL omits is empty, so a caller reads it without guarding against an unset key. A scheme and `://` are required. `mailto:someone@example.com` has neither an authority nor a host, and guessing what its parts are called would be inventing an answer rather than parsing one. The components are returned exactly as written. Percent-escapes are left alone, because decoding them here would destroy the difference between a separator and a character that merely looks like one; `dybatpho::url_decode` is there for the caller that wants it.
- [`dybatpho::url_part`](#dybatphourl_part) — Print one component of the last parsed URL.
- [`__dybatpho_network_ipv4_octets`](#__dybatpho_network_ipv4_octets) — Split an IPv4 address into its four octets as numbers. A leading zero is rejected rather than ignored. `inet_aton` and much of the software built on it read `010` as octal, so `127.0.0.010` is one host to one parser and another host to the next. An address that means two things is not an address this library will agree to.
- [`__dybatpho_network_ipv6_groups`](#__dybatpho_network_ipv6_groups) — Expand an IPv6 address into its eight groups as numbers. Everything an IPv6 address may leave out is put back here: the `::` that stands for a run of zero groups, and the dotted IPv4 tail that occupies the last two groups of a mapped address. Comparing addresses is only simple once both are written out in full. A zone index such as `%eth0` is rejected. It names an interface rather than a part of the address, and it is not comparable between two hosts.
- [`dybatpho::is_ipv4`](#dybatphois_ipv4) — Return success when a value is an IPv4 address.
- [`dybatpho::is_ipv6`](#dybatphois_ipv6) — Return success when a value is an IPv6 address.
- [`dybatpho::ip_version`](#dybatphoip_version) — Print which version of IP an address is.
- [`__dybatpho_network_parse_cidr`](#__dybatpho_network_parse_cidr) — Split a CIDR block into its address and prefix length.
- [`dybatpho::is_cidr`](#dybatphois_cidr) — Return success when a value is a CIDR block.
- [`dybatpho::cidr_netmask`](#dybatphocidr_netmask) — Print the dotted-decimal subnet mask of an IPv4 prefix length. There is no dotted form of an IPv6 prefix, so this is IPv4 only: the notation itself does not exist for the other version rather than being left out here.
- [`dybatpho::cidr_contains`](#dybatphocidr_contains) — Return success when an address falls inside a CIDR block. Both versions are supported, and an address is never inside a block of the other version: `::ffff:10.0.0.1` and `10.0.0.1` name the same host to some software, but they are not the same address and this does not pretend otherwise.
- [`dybatpho::port_open`](#dybatphoport_open) — Return success when a TCP port accepts a connection. The connection is made with Bash's own `/dev/tcp`, so nothing has to be installed for this to work. A build of Bash compiled without network redirections cannot do it, and reports the port as closed. The host and the port are passed to the timed-out shell as arguments rather than spliced into the script it runs, so a host name is never read as code.
- [`dybatpho::wait_port`](#dybatphowait_port) — Wait until a TCP port accepts a connection. This is the wait a script does after starting a service and before using it, written once. Each attempt is given no more time than the wait has left, so the whole call keeps to its budget rather than overrunning it by the length of one connection attempt.

<a id="tips"></a>
## 💡 Tips

### `dybatpho::curl_do`

- The request body is written to the provided output file, or `/dev/null` when omitted

### `dybatpho::curl_download`

- The destination directory is created automatically before downloading

### `dybatpho::curl_upload`

- The request method defaults to `POST`; pass `--request PUT` (or similar) afterwards to override it

### `dybatpho::curl_resume_download`

- A partially downloaded destination file is resumed instead of restarted

### `dybatpho::curl_request`

- Response headers aren't captured while `DRY_RUN` is enabled

### `dybatpho::curl_timeout`

- Overrides apply only for the duration of this call; global `DYBATPHO_CURL_*` timeouts are unaffected

### `dybatpho::circuit_breaker`

- The command is executed with `eval`, so pass it as one shell command string

<a id="reference"></a>
## 📚 Reference

### `__dybatpho_network_config_escape`

Escape a value for a double-quoted `curl` config parameter.
  `curl` reads a config file as `name = "value"`, where the value takes
  backslash escapes, so a backslash or a quote inside a header has to be
  escaped or it ends the value early.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Raw value |

**📤 Output on stdout**

- The escaped value, without its surrounding quotes


---

### `__dybatpho_network_secret_config`

Write the secret headers of the current request into a private
  config file for `curl --config`.


  The file is created under `umask 077` before anything is written to it, so
  the credential is never on disk in a mode another account could read, and it
  is removed as soon as the request is over. The path is an argument, which is
  public; the contents are not.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Name of the variable receiving the config file path |

**🧩 Variable sets**

- **`The`**: named variable

**🚦 Exit codes**

- `0`: A config file was written, or there was nothing to write
- `1`: Stop the script when the file cannot be created


---

### `__dybatpho_network_get_http_code`

Get description of HTTP status code

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Status code |

**📤 Output on stdout**

- Description of status code


---

### `dybatpho::curl_do`

Transferring data with URL by curl

**🧪 Example**

```bash
dybatpho::curl_do https://example.com /tmp/1
dybatpho::curl_do https://example.com /tmp/1 --compressed

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL |
| `$2` | string | Location of curl output, default is `/dev/null` |
| `$3` | string | Other options/arguments for curl |

**🌍 Environment variables**

| Variable | Type | Description |
| --- | --- | --- |
| **`DYBATPHO_CURL_MAX_RETRIES`** | number | Override the retry budget used around curl requests |

**📝 Notes**

- HTTP 4xx responses are treated as completed requests and returned to the caller as exit code `4`

**🚦 Exit codes**

- `0`: Transferred data
- `1`: Unknown error
- `3`: First digit of HTTP error code 3xx
- `4`: First digit of HTTP error code 4xx
- `5`: First digit of HTTP error code 5xx
- `127`: Curl isn't installed


---

### `dybatpho::curl_download`

Download file

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL |
| `$2` | string | Destination of file to download |
| `$@` | string | Other options/arguments for curl |

**🚦 Exit codes**

- `6`: Can't create folder of destination file

**🔗 See also**

- [dybatpho::curl_do](#dybatphocurl_do)


---

### `dybatpho::curl_json`

Transfer JSON data with URL by curl.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL |
| `$2` | string | Location of curl output, default is `/dev/null` |
| `$@` | string | Other options/arguments for curl |

**🚦 Exit codes**

- `0`: Transferred data

**🔗 See also**

- [dybatpho::curl_do](#dybatphocurl_do)


---

### `dybatpho::curl_head`

Fetch only HTTP headers for a URL by curl.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL |
| `$2` | string | Location of curl output, default is `/dev/null` |
| `$@` | string | Other options/arguments for curl |

**🚦 Exit codes**

- `0`: Transferred headers

**🔗 See also**

- [dybatpho::curl_do](#dybatphocurl_do)


---

### `dybatpho::curl_upload`

Upload fields and files with curl using multipart/form-data.

**🧪 Example**

```bash
dybatpho::curl_upload https://example.com/upload /tmp/resp.json note="nightly run" report=@/tmp/report.csv
dybatpho::curl_upload https://example.com/upload /tmp/resp.json report=@/tmp/report.csv --request PUT

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL |
| `$2` | string | Location of curl output, default is `/dev/null` |
| `$@` | string | Form fields as `name=value` or `name=@path` pairs, plus any other curl options/arguments |

**🚦 Exit codes**

- `2`: A `name=@path` field references a file that does not exist

**🔗 See also**

- [dybatpho::curl_do](#dybatphocurl_do)


---

### `dybatpho::verify_checksum`

Verify a downloaded file against an expected checksum.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | File to verify |
| `$2` | string | Expected checksum as `algorithm:hexdigest`, algorithm is one of `sha256`, `sha1`, or `md5` |

**🚦 Exit codes**

- `7`: Checksum mismatch
- `8`: Unsupported algorithm, invalid spec, or the checksum tool isn't installed


---

### `dybatpho::curl_resume_download`

Download a file with resume support and optional checksum verification.

**🧪 Example**

```bash
dybatpho::curl_resume_download https://example.com/big.iso /tmp/big.iso
dybatpho::curl_resume_download https://example.com/big.iso /tmp/big.iso sha256:3a7bd3e2360a3d...

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL |
| `$2` | string | Destination of file to download |
| `$3` | string | Optional checksum as `algorithm:hexdigest` (sha256, sha1, or md5) |
| `$@` | string | Other options/arguments for curl |

**🚦 Exit codes**

- `6`: Can't create folder of destination file
- `7`: Checksum verification failed
- `8`: Unsupported checksum algorithm or missing checksum tool

**🔗 See also**

- [dybatpho::curl_download](#dybatphocurl_download)
- [dybatpho::verify_checksum](#dybatphoverify_checksum)


---

### `dybatpho::curl_parse_response`

Parse a raw curl header dump (and optional body file) into normalized response state.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Path to a header file captured via `curl -D` (may contain multiple header blocks from redirects; the last block wins) |
| `$2` | string | Optional path to the response body file to record |

**🧩 Variable sets**

- **`DYBATPHO_HTTP_STATUS`**: number Status code of the last received response block
- **`DYBATPHO_HTTP_HEADERS`**: map Lower-cased header name to value, from the last response block
- **`DYBATPHO_HTTP_BODY_FILE`**: string Path to the response body, or empty when omitted

**🚦 Exit codes**

- `1`: No status line was found in the header file


---

### `dybatpho::curl_response_header`

Print a normalized response header captured by `dybatpho::curl_parse_response`.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Header name, matched case-insensitively |
| `$2` | string | Optional default value |

**📤 Output on stdout**

- Header value

**🚦 Exit codes**

- `1`: Header is missing and no default was supplied


---

### `dybatpho::curl_request`

Perform a request via `dybatpho::curl_do` and parse its response into normalized status/header/body state.

**🧪 Example**

```bash
dybatpho::curl_request https://example.com/api /tmp/resp.json
echo "${DYBATPHO_HTTP_STATUS}"
dybatpho::curl_response_header content-type

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL |
| `$2` | string | Location of curl output, default is `/dev/null` |
| `$@` | string | Other options/arguments for curl |

**🧩 Variable sets**

- **`DYBATPHO_HTTP_STATUS`**: number Status code of the last received response block
- **`DYBATPHO_HTTP_HEADERS`**: map Lower-cased header name to value, from the last response block
- **`DYBATPHO_HTTP_BODY_FILE`**: string Path holding the response body

**🔗 See also**

- [dybatpho::curl_do](#dybatphocurl_do)
- [dybatpho::curl_parse_response](#dybatphocurl_parse_response)


---

### `dybatpho::curl_timeout`

Perform a request via `dybatpho::curl_do` with connect/total timeouts scoped to this call only.

**🧪 Example**

```bash
dybatpho::curl_timeout https://example.com /tmp/out 2 10
dybatpho::curl_timeout https://example.com /tmp/out "" 5 --header "X-Test: 1"

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL |
| `$2` | string | Location of curl output, default is `/dev/null` |
| `$3` | number | Connect timeout in seconds for this request, empty keeps the global default |
| `$4` | number | Total timeout in seconds for this request, empty keeps the global default |
| `$@` | string | Other options/arguments for curl |

**🔗 See also**

- [dybatpho::curl_do](#dybatphocurl_do)


---

### `dybatpho::circuit_state`

Report whether a circuit breaker key is currently open, half-open, or closed.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Circuit key |

**📤 Output on stdout**

- `open`, `half-open`, or `closed`


---

### `dybatpho::circuit_reset`

Reset a circuit breaker key back to the closed state.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Circuit key |


---

### `dybatpho::circuit_breaker`

Run a shell command guarded by a circuit breaker keyed by name.

**🧪 Example**

```bash
dybatpho::circuit_breaker api.example.com "dybatpho::curl_do https://api.example.com/health"

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Circuit key, typically a host or service name |
| `$2` | string | Shell command string to run |

**🌍 Environment variables**

| Variable | Type | Description |
| --- | --- | --- |
| **`DYBATPHO_CIRCUIT_THRESHOLD`** | number | Consecutive failures before the circuit opens (default `5`) |
| **`DYBATPHO_CIRCUIT_COOLDOWN`** | number | Seconds the circuit stays open before a trial request is allowed (default `30`) |

**📝 Notes**

- Circuit breaker state is in-memory and process-local; it does not persist across script invocations

**🚦 Exit codes**

- `0`: The command succeeded, or a trial request succeeded and closed the circuit
- `9`: The circuit is open; the command was not attempted
- `other`: The command's own exit code, while the circuit is closed or half-open


---

### `__dybatpho_network_parse_authority`

Split the authority of a URL into user, password, host, and port.
  The authority is the awkward part of the grammar: everything in it is
  optional, the delimiters repeat, and an IPv6 literal carries colons of its
  own inside brackets.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Authority, such as `user:pass@host:443` or `[::1]:8080` |

**🧩 Variable sets**

- **`DYBATPHO_URL`**: The `user`, `password`, `host`, and `port` entries

**🚦 Exit codes**

- `1`: The authority names no host, or a port that is not a port


---

### `__dybatpho_network_is_port`

Return success when a value is a usable TCP or UDP port number.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Value to test |

**🚦 Exit codes**

- `0`: The value is a decimal number from 1 to 65535
- `1`: It is not


---

### `dybatpho::url_parse`

Split a URL into its components.
  The result lands in `DYBATPHO_URL`, one entry per component, the way
  `dybatpho::curl_parse_response` leaves a response in `DYBATPHO_HTTP_*`.
  Every entry is always present; a component the URL omits is empty, so a
  caller reads it without guarding against an unset key.


  A scheme and `://` are required. `mailto:someone@example.com` has neither an
  authority nor a host, and guessing what its parts are called would be
  inventing an answer rather than parsing one.


  The components are returned exactly as written. Percent-escapes are left
  alone, because decoding them here would destroy the difference between a
  separator and a character that merely looks like one; `dybatpho::url_decode`
  is there for the caller that wants it.

**🧪 Examples**

```bash
dybatpho::url_parse "https://user:secret@example.com:8443/a/b?q=1#top"
printf '%s\n' "${DYBATPHO_URL[host]}"    # example.com
printf '%s\n' "${DYBATPHO_URL[port]}"    # 8443
printf '%s\n' "${DYBATPHO_URL[path]}"    # /a/b

```

```bash
dybatpho::url_parse "http://[::1]:8080/health"
printf '%s\n' "${DYBATPHO_URL[host]}"    # ::1

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | URL to split |

**🧩 Variable sets**

- **`DYBATPHO_URL`**: map The `scheme`, `user`, `password`, `host`, `port`, `path`, `query`, and `fragment` of the URL

**🚦 Exit codes**

- `0`: The URL was split
- `1`: The URL has no scheme, no host, or a port that is not a port

**🔗 See also**

- [- `dybatpho::url_part` - `dybatpho::url_decode](#dybatphourl_part-dybatphourl_decode)


---

### `dybatpho::url_part`

Print one component of the last parsed URL.

**🧪 Example**

```bash
dybatpho::url_parse "https://example.com/health"
dybatpho::url_part host            # example.com
dybatpho::url_part port 443        # 443, the default, since none was given

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Component name: `scheme`, `user`, `password`, `host`, `port`, `path`, `query`, or `fragment` |
| `$2` | string | Optional value to print when the component is empty |

**📤 Output on stdout**

- The component, or the default

**🚦 Exit codes**

- `1`: The component is empty and no default was supplied
- `1`: Stop the script when the name is not a component of a URL

**🔗 See also**

- [- `dybatpho::url_parse](#dybatphourl_parse)


---

### `__dybatpho_network_ipv4_octets`

Split an IPv4 address into its four octets as numbers.
  A leading zero is rejected rather than ignored. `inet_aton` and much of the
  software built on it read `010` as octal, so `127.0.0.010` is one host to
  one parser and another host to the next. An address that means two things
  is not an address this library will agree to.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Address to split |
| `$2` | string | Name of the array variable receiving the four octets |

**🧩 Variable sets**

- **`The`**: named array, to four numbers from 0 to 255

**🚦 Exit codes**

- `1`: The value is not an IPv4 address


---

### `__dybatpho_network_ipv6_groups`

Expand an IPv6 address into its eight groups as numbers.
  Everything an IPv6 address may leave out is put back here: the `::` that
  stands for a run of zero groups, and the dotted IPv4 tail that occupies the
  last two groups of a mapped address. Comparing addresses is only simple once
  both are written out in full.


  A zone index such as `%eth0` is rejected. It names an interface rather than
  a part of the address, and it is not comparable between two hosts.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Address to expand |
| `$2` | string | Name of the array variable receiving the eight groups |

**🧩 Variable sets**

- **`The`**: named array, to eight numbers from 0 to 65535

**🚦 Exit codes**

- `1`: The value is not an IPv6 address


---

### `dybatpho::is_ipv4`

Return success when a value is an IPv4 address.

**🧪 Example**

```bash
dybatpho::is_ipv4 192.0.2.10      # yes
dybatpho::is_ipv4 192.0.2.256     # no
dybatpho::is_ipv4 127.0.0.010     # no, a leading zero is ambiguous

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Value to test |

**🚦 Exit codes**

- `0`: The value is an IPv4 address
- `1`: It is not


---

### `dybatpho::is_ipv6`

Return success when a value is an IPv6 address.

**🧪 Example**

```bash
dybatpho::is_ipv6 ::1                    # yes
dybatpho::is_ipv6 2001:db8::1            # yes
dybatpho::is_ipv6 ::ffff:192.0.2.1       # yes
dybatpho::is_ipv6 2001:db8::1::2         # no, one `::` is all there is

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Value to test |

**📝 Notes**

- A zone index such as `fe80::1%eth0` is refused: it names an interface rather than a part of the address

**🚦 Exit codes**

- `0`: The value is an IPv6 address
- `1`: It is not


---

### `dybatpho::ip_version`

Print which version of IP an address is.

**🧪 Example**

```bash
dybatpho::ip_version 192.0.2.10    # 4
dybatpho::ip_version ::1           # 6

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Address to inspect |

**📤 Output on stdout**

- `4` or `6`

**🚦 Exit codes**

- `1`: The value is not an IP address of either version


---

### `__dybatpho_network_parse_cidr`

Split a CIDR block into its address and prefix length.

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Block such as `10.0.0.0/8` or `2001:db8::/32` |
| `$2` | string | Name of the variable receiving the address |
| `$3` | string | Name of the variable receiving the prefix length |
| `$4` | string | Name of the variable receiving the IP version |

**🧩 Variable sets**

- **`The`**: three named variables

**🚦 Exit codes**

- `1`: The value is not a CIDR block


---

### `dybatpho::is_cidr`

Return success when a value is a CIDR block.

**🧪 Example**

```bash
dybatpho::is_cidr 10.0.0.0/8        # yes
dybatpho::is_cidr 2001:db8::/32     # yes
dybatpho::is_cidr 10.0.0.0/33       # no

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Value to test |

**🚦 Exit codes**

- `0`: The value is a CIDR block of either IP version
- `1`: It is not


---

### `dybatpho::cidr_netmask`

Print the dotted-decimal subnet mask of an IPv4 prefix length.
  There is no dotted form of an IPv6 prefix, so this is IPv4 only: the
  notation itself does not exist for the other version rather than being
  left out here.

**🧪 Example**

```bash
dybatpho::cidr_netmask 24    # 255.255.255.0
dybatpho::cidr_netmask 0     # 0.0.0.0

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Prefix length from 0 to 32, with or without a leading `/` |

**📤 Output on stdout**

- The subnet mask

**🚦 Exit codes**

- `1`: Stop the script when the value is not an IPv4 prefix length


---

### `dybatpho::cidr_contains`

Return success when an address falls inside a CIDR block.
  Both versions are supported, and an address is never inside a block of the
  other version: `::ffff:10.0.0.1` and `10.0.0.1` name the same host to some
  software, but they are not the same address and this does not pretend
  otherwise.

**🧪 Example**

```bash
dybatpho::cidr_contains 10.0.0.0/8 10.1.2.3            # yes
dybatpho::cidr_contains 10.0.0.0/8 11.1.2.3            # no
dybatpho::cidr_contains 2001:db8::/32 2001:db8::1      # yes
dybatpho::cidr_contains 0.0.0.0/0 203.0.113.1          # yes

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | CIDR block |
| `$2` | string | Address to test |

**🚦 Exit codes**

- `0`: The address is inside the block
- `1`: It is not
- `1`: Stop the script when the block or the address is malformed


---

### `dybatpho::port_open`

Return success when a TCP port accepts a connection.
  The connection is made with Bash's own `/dev/tcp`, so nothing has to be
  installed for this to work. A build of Bash compiled without network
  redirections cannot do it, and reports the port as closed.


  The host and the port are passed to the timed-out shell as arguments rather
  than spliced into the script it runs, so a host name is never read as code.

**🧪 Example**

```bash
dybatpho::port_open localhost 5432
dybatpho::port_open db.internal 5432 2

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Host name or address |
| `$2` | number | Port |
| `$3` | number | Seconds to wait, defaulting to `DYBATPHO_PORT_TIMEOUT` |

**🌍 Environment variables**

| Variable | Type | Description |
| --- | --- | --- |
| **`DYBATPHO_PORT_TIMEOUT`** | number | Seconds to wait for the connection |

**📝 Notes**

- The timeout needs the `timeout` command; without it the connection waits as long as the system's own TCP timeout

**🚦 Exit codes**

- `0`: The port accepted a connection
- `1`: It did not, within the timeout
- `1`: Stop the script when the port or the timeout is not a number

**🔗 See also**

- [- `dybatpho::wait_port](#dybatphowait_port)


---

### `dybatpho::wait_port`

Wait until a TCP port accepts a connection.
  This is the wait a script does after starting a service and before using it,
  written once. Each attempt is given no more time than the wait has left, so
  the whole call keeps to its budget rather than overrunning it by the length
  of one connection attempt.

**🧪 Example**

```bash
docker compose up -d
dybatpho::wait_port localhost 5432 60 \
  || dybatpho::die "The database never came up"

```

**🧾 Arguments**

| Name | Type | Description |
| --- | --- | --- |
| `$1` | string | Host name or address |
| `$2` | number | Port |
| `$3` | number | Seconds to keep trying, defaulting to `DYBATPHO_WAIT_PORT_TIMEOUT` |
| `$4` | number | Seconds between attempts, defaulting to `DYBATPHO_WAIT_PORT_INTERVAL` |

**🌍 Environment variables**

| Variable | Type | Description |
| --- | --- | --- |
| **`DYBATPHO_WAIT_PORT_TIMEOUT`** | number | Seconds to keep trying |
| **`DYBATPHO_WAIT_PORT_INTERVAL`** | number | Seconds between attempts |

**🚦 Exit codes**

- `0`: The port accepted a connection before the time ran out
- `1`: It never did
- `1`: Stop the script when an argument is not a number

**🔗 See also**

- [- `dybatpho::port_open](#dybatphoport_open)

