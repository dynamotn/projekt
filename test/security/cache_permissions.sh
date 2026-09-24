#!/usr/bin/env bash
# PoC 3 -- dybatpho::file_write_atomic creates its staging file under the
# caller's umask, so a *new* file comes out world-readable on a normal
# umask 022 host. That is the path dybatpho::cache_set uses, and the ai module
# caches provider responses through it.
#
# The library already knows how to do this properly: dybatpho::secret_write_file
# sets `umask 077` around the write. The inconsistency is the finding.
set -uo pipefail

ROOT="${1:?usage: poc3_cache_mode.sh <repo-root> <workdir>}"
WORK="${2:?usage: poc3_cache_mode.sh <repo-root> <workdir>}"

rm -rf "${WORK}"
mkdir -p "${WORK}"

export LOG_LEVEL=fatal
export DYBATPHO_CACHE_DIR="${WORK}/cache"
export DYBATPHO_AI_CACHE_DIR="${WORK}/aicache"

umask 022

# shellcheck disable=SC1090
source "${ROOT}/init.sh" --modules cache

mode_of() {
  stat -c '%a' "$1" 2> /dev/null || stat -f '%Lp' "$1" 2> /dev/null
}

printf 'sk-live-PROVIDER-RESPONSE-WITH-CUSTOMER-DATA\n' | dybatpho::cache_set answer > /dev/null

CACHE_FILE="$(dybatpho::cache_path answer)"
CACHE_DIR="$(dybatpho::cache_dir)"

echo "== cache file =="
echo "    path: ${CACHE_FILE}"
FILE_MODE="$(mode_of "${CACHE_FILE}")"
DIR_MODE="$(mode_of "${CACHE_DIR}")"
echo "    file mode: ${FILE_MODE}"
echo "    dir  mode: ${DIR_MODE}"
echo "    contents : $(head -c 60 "${CACHE_FILE}")"

echo
status=0
if ((8#${FILE_MODE} & 8#077)); then
  echo "VULNERABLE: the cache file is readable beyond its owner (mode ${FILE_MODE})"
  status=1
fi
if ((8#${DIR_MODE} & 8#077)); then
  echo "VULNERABLE: the cache directory is traversable beyond its owner (mode ${DIR_MODE})"
  status=1
fi
if ((status == 0)); then
  echo "OK: cache file and directory are owner-only"
fi

# For contrast: the module that already gets this right.
echo
echo "== contrast: dybatpho::secret_write_file =="
# shellcheck disable=SC2034 # read by name through a nameref in secret_write_file
SECRET_VALUE="top-secret-value"
dybatpho::load secret 2> /dev/null || true
dybatpho::secret_write_file "${WORK}/secret.txt" SECRET_VALUE
echo "    secret file mode: $(mode_of "${WORK}/secret.txt")"

exit "${status}"
