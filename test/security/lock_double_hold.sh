#!/usr/bin/env bash
# PoC / regression -- can two processes hold the same dybatpho lock at once?
#
# The original defect: lock_acquire claimed with `mkdir` and wrote the owner's
# pid afterwards, so between those two steps the lock existed with nobody in it.
# A second process read the missing pid as "nobody holds this", removed the lock
# and took it. Both then believed they held it.
#
# The fix claims with `ln -s`, carrying the pid in the link target, so the lock
# can never exist without an owner. That makes the original window impossible to
# model directly, so this script tests the property the window broke instead:
#
#   1. real contention -- N processes racing over a non-atomic counter
#   2. a lock is never observable without an owner
set -uo pipefail

ROOT="${1:?usage: lock_double_hold.sh <repo-root> <workdir>}"
WORK="${2:?usage: lock_double_hold.sh <repo-root> <workdir>}"

rm -rf "${WORK}"
mkdir -p "${WORK}"

export LOG_LEVEL=fatal
export DYBATPHO_LOCK_DIR="${WORK}"

# shellcheck disable=SC1090
source "${ROOT}/init.sh" --modules lock

status=0

# --------------------------------------------------------------------------
# 1. Real contention over a critical section.
# --------------------------------------------------------------------------
WORKERS=8
ROUNDS=3
COUNTER="${WORK}/counter"
printf '0' > "${COUNTER}"

cat > "${WORK}/worker.sh" << 'WORKER'
#!/usr/bin/env bash
set -uo pipefail
ROOT="$1"; WORK="$2"; ROUNDS="$3"
export LOG_LEVEL=fatal
export DYBATPHO_LOCK_DIR="${WORK}"
# shellcheck disable=SC1090
source "${ROOT}/init.sh" --modules lock
for ((round = 0; round < ROUNDS; round++)); do
  dybatpho::lock_acquire "contended" 60 > /dev/null 2>&1 || exit 1
  # A read-modify-write with a gap in the middle. If the lock excludes, every
  # increment survives; if it does not, some are lost.
  value="$(cat "${WORK}/counter")"
  sleep 0.01
  printf '%s' "$((value + 1))" > "${WORK}/counter"
  dybatpho::lock_release "contended" > /dev/null 2>&1
done
WORKER
chmod +x "${WORK}/worker.sh"

echo "== ${WORKERS} processes, ${ROUNDS} critical sections each =="
pids=()
for ((worker = 0; worker < WORKERS; worker++)); do
  bash "${WORK}/worker.sh" "${ROOT}" "${WORK}" "${ROUNDS}" &
  pids+=("$!")
done
for pid in "${pids[@]}"; do
  wait "${pid}" 2> /dev/null || true
done

expected=$((WORKERS * ROUNDS))
actual="$(cat "${COUNTER}")"
echo "    increments expected: ${expected}"
echo "    increments recorded: ${actual}"
if ((actual != expected)); then
  echo "    LOST $((expected - actual)) increments -- the lock did not exclude"
  status=1
else
  echo "    none lost"
fi

# --------------------------------------------------------------------------
# 2. A lock is never observable without an owner.
# --------------------------------------------------------------------------
echo
echo "== the lock always names its owner =="
dybatpho::lock_acquire "owned" 5 > /dev/null 2>&1
owner="$(dybatpho::lock_field "$(dybatpho::lock_path "owned")" pid)"
if [[ -z "${owner}" ]]; then
  echo "    the lock exists with no pid in it"
  status=1
else
  echo "    pid recorded at claim time: ${owner} (this process is $$)"
fi
dybatpho::lock_release "owned" > /dev/null 2>&1

# Releasing on interrupt is covered in test/lock.bats instead. Whether an
# uninstrumented shell survives a signal long enough to reach its own release
# line is timing-dependent -- the old code often did -- so an end-to-end
# interrupt here flaps rather than proving anything. The bats test asserts the
# handler is installed while the command runs and gone afterwards, which is the
# part that actually changed.

echo
if ((status == 0)); then
  echo "OK: the lock excludes and always names its owner"
  exit 0
fi
echo "VULNERABLE: see above"
exit 1
