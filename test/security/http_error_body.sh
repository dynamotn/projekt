#!/usr/bin/env bash
# PoC -- an API's explanation of why it said no never reaches the caller.
#
# dybatpho::curl_do always passes -f (--fail) to curl. With -f, curl writes no
# body for an HTTP error: it returns exit 22 and discards the response. So when
# GitHub answers
#
#   422 {"message":"Validation Failed","errors":[{"field":"title", ...}]}
#
# the library reports "HTTP 422" and the part that says which field was wrong is
# gone. Every forge_* failure path is written as
#
#   || dybatpho::die "Could not create issue '${title}' (HTTP ${DYBATPHO_HTTP_STATUS})"
#
# which is the whole diagnosis a user gets.
#
# This runs a real local HTTP server, so the behaviour is curl's own rather than
# a stub's.
set -uo pipefail

ROOT="${1:?usage: http_error_body.sh <repo-root> <workdir>}"
WORK="${2:?usage: http_error_body.sh <repo-root> <workdir>}"

rm -rf "${WORK}"
mkdir -p "${WORK}"

command -v python3 > /dev/null 2>&1 || {
  echo "INCONCLUSIVE: python3 is needed to serve the error response"
  exit 2
}

ERROR_BODY='{"message":"Validation Failed","errors":[{"field":"title","code":"missing"}]}'

cat > "${WORK}/server.py" << 'PY'
import http.server
import sys

body = sys.argv[2].encode()
port_file = sys.argv[1]


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(422)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


# Port 0 lets the kernel choose, which avoids probing for a free one.
server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
with open(port_file, "w") as handle:
    handle.write(str(server.server_address[1]))
server.serve_forever()
PY

PORT_FILE="${WORK}/port"
python3 "${WORK}/server.py" "${PORT_FILE}" "${ERROR_BODY}" &
SERVER_PID=$!

PORT=""
for _ in $(seq 1 100); do
  if [[ -s "${PORT_FILE}" ]]; then
    PORT="$(cat "${PORT_FILE}")"
    break
  fi
  sleep 0.05
done
if [[ -z "${PORT}" ]]; then
  kill "${SERVER_PID}" 2> /dev/null
  echo "INCONCLUSIVE: the test server never reported a port"
  exit 2
fi

export LOG_LEVEL=fatal
export DYBATPHO_CURL_MAX_RETRIES=0

# shellcheck disable=SC1090
source "${ROOT}/init.sh" --modules network

BODY_FILE="${WORK}/body"
: > "${BODY_FILE}"
status=0
dybatpho::curl_do "http://127.0.0.1:${PORT}/issues" "${BODY_FILE}" > /dev/null 2>&1 || status=$?

# `init.sh` turned on `set -e`, and waiting on a process we just killed reports
# its 143. Neither is a failure of the test.
kill "${SERVER_PID}" 2> /dev/null || true
wait "${SERVER_PID}" 2> /dev/null || true

echo "== what the server said =="
echo "    422 ${ERROR_BODY}"
echo
echo "== what the caller can see =="
echo "    curl_do exit code: ${status}"
echo "    body file size:    $(wc -c < "${BODY_FILE}" | tr -d ' ') bytes"
if [[ -s "${BODY_FILE}" ]]; then
  echo "    body file holds:   $(head -c 120 "${BODY_FILE}")"
fi

echo
if [[ -s "${BODY_FILE}" ]] && grep -q 'Validation Failed' "${BODY_FILE}"; then
  echo "OK: the error body reached the caller"
  exit 0
fi
echo "VULNERABLE: the response body was discarded; only the status survived"
echo "  a caller cannot tell a bad field from a bad token from a rate limit"
exit 1
