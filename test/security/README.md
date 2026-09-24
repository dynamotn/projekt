# Security proofs of concept

Each script here demonstrates one weakness that was found and fixed, and is
written so that it **fails against the vulnerable code and passes against the
fixed code**. They are kept because a regression here is quiet: nothing breaks,
a credential is simply readable again.

The Bats suite covers the same properties as ordinary tests — see
`test/network.bats`, `test/ai.bats` and `test/cache.bats`. These scripts exist
alongside them because they demonstrate the *attack*, not just the invariant,
which is what makes the risk legible to a reviewer.

## Running them

Each takes the repository root and a scratch directory, and exits non-zero when
the weakness is present:

```sh
bash test/security/token_in_argv.sh      "$PWD" /tmp/poc-token
bash test/security/ai_state_symlink.sh   "$PWD" /tmp/poc-state
bash test/security/cache_permissions.sh  "$PWD" /tmp/poc-cache
```

They make no network requests: `token_in_argv.sh` puts a stub `curl` on `PATH`
that reports its own `/proc/<pid>/cmdline`, which is the same bytes `ps auxww`
would show another user on the host.

## What each one covers

| Script | Weakness |
| --- | --- |
| `token_in_argv.sh` | A bearer token or API key passed to `curl` as `--header` is readable by every account on the host through `/proc/<pid>/cmdline`. Also asserts the credential still *arrives*, in a `0600` config file, so a "fix" that merely drops the header fails. |
| `ai_state_symlink.sh` | The `ai` counter file defaulted to a predictable name in a shared `/tmp` and was written with a plain `>`, which follows a symlink — an arbitrary-file-overwrite primitive. |
| `cache_permissions.sh` | Cache entries were created under the caller's umask, so `0644` in a `0755` directory on a normal host, while holding whatever was expensive to fetch. |

## A note on `ai_state_symlink.sh`

It models the attacker having already won the race for the predictable name, and
then measures only the consequence: whether the library writes through the link.
Pre-creating the name for a range of pids is reliable in practice, because pids
are public and drawn from a small space — that part is not what the script is
trying to prove.
