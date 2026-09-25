# Proposal: make `dybatpho::lock_acquire` actually exclude

Status: proposed, not implemented. The evidence below is reproducible today.

## Problem

`dybatpho::lock_acquire` claims the lock and then records who claimed it, in two
steps that are not atomic together:

```sh
if mkdir "${lock_path}"; then           # 1. claim it
  printf '%s' "$$" > "${lock_path}/pid" # 2. say who claimed it
```

Between those lines the directory exists with no `pid` file in it. A second
process arriving in that window calls `dybatpho::lock_reclaim_stale`, which asks
`dybatpho::lock_is_alive`, which reads the missing `pid`:

```sh
pid="$(dybatpho::lock_field "${lock_path}" pid)"
[[ -n "${pid}" ]] || return 1          # no pid -> "nobody holds this"
```

It concludes the lock is stale, `rm -rf`s the directory out from under the first
process, and its own `mkdir` then succeeds. Both processes proceed believing
they hold the lock, and nothing ever tells either of them otherwise.

The window is small but it is not theoretical: it spans a `mkdir` syscall, a
`$$` expansion and a file creation, and it is entered by every single acquire.
Under contention — which is the only time a lock matters — several processes are
in `lock_acquire` at once by construction.

## Evidence

`test/security/lock_double_hold.sh` models the window rather than racing for it:
it leaves the lock directory exactly as step 1 leaves it, then asks a second
process to acquire the same lock.

```
$ bash test/security/lock_double_hold.sh "$PWD" /tmp/poc-lock
== process A has claimed the lock but not yet written its pid ==
    /tmp/poc-lock/dybatpho-poc.lock exists
    no pid file in it yet, which is the whole window

== process B asks for the same lock, with no patience at all ==
    B acquired it: two processes now believe they hold 'poc'
    the lock now names pid 3149064, which is B
    A still thinks it holds the lock, and nothing will tell it otherwise

VULNERABLE: the lock does not exclude
```

A second, smaller defect sits next to it: `dybatpho::with_lock` releases the
lock on the normal path only.

```sh
dybatpho::lock_acquire "${name}" "${timeout}" || return 1
"$@" || exit_code=$?
dybatpho::lock_release "${name}"
```

There is no trap, so `Ctrl-C` during the command leaves the lock behind. It is
then reclaimed only once the holder's pid is seen to be gone — which is the
mechanism the first defect already shows to be unsound.

## Options

### A. Treat "claimed but unnamed" as held, with a grace period

`lock_is_alive` returns true when the directory exists but has no `pid` yet,
and `lock_reclaim_stale` only reclaims a directory older than
`DYBATPHO_LOCK_GRACE` seconds.

- Smallest change; on-disk format unchanged.
- A process that dies *between* the two steps leaves a lock that is only
  reclaimed after the grace period, so the grace has to be short enough to not
  strand a queue and long enough to cover a slow acquire. There is no value that
  is right on every host, which is the usual sign of a workaround.

### B. Make the claim and the identity one atomic operation (recommended)

Claim the lock with `ln -s`, carrying the identity in the link target:

```sh
ln -s "${pid}:${host}:${epoch}" "${lock_path}"
```

`symlink()` is atomic and fails when the name already exists, so there is no
window at all: whoever creates the link holds the lock, and the link already
says who they are. `readlink` reads it back.

- No grace period, no tuning, nothing to get wrong under load.
- The on-disk representation changes from a directory to a symbolic link. That
  is internal — `lock_info`, `lock_is_held` and `lock_field` keep their current
  output — but anything that inspected the lock directory by hand stops working,
  and a lock written by an older version is not recognised by a newer one. The
  changelog has to say so.
- `command`, which can hold anything, does not belong in a link target. It moves
  to a sidecar file written after the claim, and is reported as empty when it is
  missing — it is diagnostic only, so a missing one is not a correctness
  problem.

### C. `flock(1)`

Correct and boring, but it is Linux/util-linux, and the module exists precisely
to be portable to macOS and BusyBox. It could be used opportunistically when
present, which adds a second code path to test for no benefit on the hosts that
need the fallback anyway.

## Recommendation

**B**, plus a trap in `with_lock`:

```sh
dybatpho::trap "dybatpho::lock_release '${name}'" EXIT HUP INT TERM
```

registered after a successful acquire, so an interrupted command releases what
it took. The trap must be scoped to the acquiring shell, the way
`dybatpho::cleanup_file_on_exit` already handles a subshell.

## What "done" looks like

- `test/security/lock_double_hold.sh` exits 0.
- A new test acquires from two real background processes in a loop and asserts
  the lock is never held twice.
- A test interrupts a `with_lock` command and asserts the lock is gone.
- `doc/spec/lock.md` gains the requirements for atomic claim, for reclaiming a
  lock whose holder is gone, and for release on interrupt; each with an `IT-`
  entry pointing at a real test.
- `CHANGELOG.md` records the on-disk format change under `### Changed` and the
  race under `### Fixed`.
