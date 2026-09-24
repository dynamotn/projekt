#!/usr/bin/env bash
# PoC 1 -- a bearer token / API key reaches curl as a command-line argument,
# so every other user on the host can read it out of /proc/<pid>/cmdline,
# which is exactly what `ps auxww` prints.
#
# The stub curl stands in for "another user running ps": it reads its own argv
# back out of the kernel, the same bytes ps would show.
#
# No EXIT trap here on purpose: bash runs an inherited EXIT trap when a `$( )`
# subshell exits too, and the library uses plenty of those.
set -uo pipefail

ROOT="${1:?usage: poc1_token_argv.sh <repo-root>}"
WORK="${2:?usage: poc1_token_argv.sh <repo-root> <workdir>}"

rm -rf "${WORK}"
mkdir -p "${WORK}/bin" "${WORK}/repo"

SECRET_TOKEN="ghp_S3CRET_TOKEN_DO_NOT_LEAK_0123456789"
ARGV_DUMP="${WORK}/argv.txt"

cat > "${WORK}/bin/curl" << 'STUB'
#!/usr/bin/env bash
# Read our own argv exactly as ps/proc exposes it to any user on the host.
if [[ -r "/proc/$$/cmdline" ]]; then
  tr "\\0" "\\n" < "/proc/$$/cmdline" >> "${POC_ARGV_DUMP}"
else
  printf '%s\n' "$@" >> "${POC_ARGV_DUMP}"
fi
# Also record anything handed over on stdin, so a fix that moves the header
# there is visible as a fix rather than as the header disappearing.
if [[ ! -t 0 ]]; then
  { printf '<<<STDIN>>>\n'; cat; } >> "${POC_ARGV_DUMP}" 2>/dev/null || true
fi
args=("$@")
# Record what arrived out of band, so a fix is visibly a fix (credential still
# delivered, just privately) rather than the header having gone missing.
for ((i = 0; i < ${#args[@]}; i++)); do
  if [[ "${args[i]}" == "--config" ]]; then
    cfg="${args[i + 1]}"
    mode="$(stat -c '%a' "${cfg}" 2>/dev/null || stat -f '%Lp' "${cfg}" 2>/dev/null)"
    { printf 'CONFIG %s mode=%s\n' "${cfg}" "${mode}"; cat "${cfg}"; } >> "${POC_OOB_DUMP}"
  fi
done
for ((i = 0; i < ${#args[@]}; i++)); do
  if [[ "${args[i]}" == "-o" ]]; then
    printf '{"id":1,"number":1,"tag_name":"v1.0.0"}' > "${args[i + 1]}" 2>/dev/null || true
  fi
  if [[ "${args[i]}" == "-D" ]]; then
    printf 'HTTP/1.1 200 OK\r\n\r\n' > "${args[i + 1]}" 2>/dev/null || true
  fi
done
printf '200'
exit 0
STUB
chmod +x "${WORK}/bin/curl"

export POC_ARGV_DUMP="${ARGV_DUMP}"
export POC_OOB_DUMP="${WORK}/oob.txt"
: > "${ARGV_DUMP}"
: > "${WORK}/oob.txt"
export PATH="${WORK}/bin:${PATH}"

git -C "${WORK}/repo" init -q
git -C "${WORK}/repo" remote add origin https://github.com/example/project.git
cd "${WORK}/repo" || exit 1

# shellcheck disable=SC1090
source "${ROOT}/init.sh" --modules forge ai

export LOG_LEVEL=fatal
export DYBATPHO_FORGE_TOKEN="${SECRET_TOKEN}"

echo "== exercising forge (GitHub REST) =="
dybatpho::forge_request GET "issues" "" /dev/null > /dev/null 2>&1 || true

echo "== exercising ai (Anthropic messages) =="
export DYBATPHO_AI_PROVIDER=anthropic
export DYBATPHO_AI_API_KEY="${SECRET_TOKEN}"
dybatpho::ai_ask "hello" > /dev/null 2>&1 || true

echo
echo "== what another user would see in 'ps auxww' =="
if [[ ! -s "${ARGV_DUMP}" ]]; then
  echo "INCONCLUSIVE: the stub curl was never invoked"
  exit 2
fi

# Only argv counts. Anything after a <<<STDIN>>> marker was piped, not exposed.
argv_only="${WORK}/argv_only.txt"
awk '/^<<<STDIN>>>$/ {skip = 1; next} /^\/.*\/curl$|^curl$/ {skip = 0} skip != 1' \
  "${ARGV_DUMP}" > "${argv_only}"

if grep -q -- "${SECRET_TOKEN}" "${argv_only}"; then
  echo "VULNERABLE: the token appears in curl's argv"
  grep -n -- "${SECRET_TOKEN}" "${argv_only}" | sed 's/^/    /'
  exit 1
fi

echo "OK: the token never appears in curl's argv"
echo "    argv lines captured: $(wc -l < "${argv_only}")"

echo
echo "== but it still reached curl, out of band =="
if [[ -s "${WORK}/oob.txt" ]] && grep -q -- "${SECRET_TOKEN}" "${WORK}/oob.txt"; then
  grep -E 'CONFIG |header = ' "${WORK}/oob.txt" \
    | sed "s/${SECRET_TOKEN}/<the token>/" | sed 's/^/    /'
  bad=0
  while read -r _ _ modefield; do
    mode="${modefield#mode=}"
    if ((8#${mode} & 8#077)); then
      echo "    VULNERABLE: config file mode ${mode} is readable by others"
      bad=1
    fi
  done < <(grep '^CONFIG ' "${WORK}/oob.txt")
  ((bad == 0)) && echo "    config file is owner-only"
  exit "${bad}"
fi
echo "    WARNING: the credential was not delivered at all -- that is a broken fix"
exit 3
