# Proposal: keep the body of an HTTP error

Status: proposed, not implemented. The evidence below is reproducible today.

## Problem

`dybatpho::curl_do` always passes `-f`:

```sh
local curl_args=(-fsSL -D "${header_file}" -w '%{http_code}' -o "${output}")
```

`curl -f` is documented to "fail fast with no output on server errors". It does
exactly that: on a 4xx or 5xx it returns exit 22 and writes **nothing** to `-o`
— it does not even create the file. The response body is gone before the
library ever sees it.

That body is where APIs put the answer to "why not". GitHub replies to a bad
issue with

```json
422 {"message":"Validation Failed","errors":[{"field":"title","code":"missing"}]}
```

and every `forge_*` failure path in the library reduces it to:

```sh
|| dybatpho::die "Could not create issue '${title}' (HTTP ${DYBATPHO_HTTP_STATUS})"
```

So a user is told `HTTP 422` and nothing else. A bad field, an expired token, a
rate limit and a repository that does not exist are all indistinguishable.

There is a second loss in the same three lines. The status is captured from
`-w '%{http_code}'` on curl's stdout, but because `-f` makes curl exit non-zero
the capture is discarded:

```sh
code=$(command curl "${curl_args[@]}" "${url}") || {
  code="000"
  dybatpho::error "Error when access ${url}"
}
```

`curl_do` therefore returns 1 — its "unknown" branch — for a 422, rather than
the documented 4. Callers that branch on the exit code cannot see the class of
the error either.

## Evidence

`test/security/http_error_body.sh` serves a real 422 from a local HTTP server,
so the behaviour is curl's own rather than a stub's:

```
$ bash test/security/http_error_body.sh "$PWD" /tmp/poc-http
== what the server said ==
    422 {"message":"Validation Failed","errors":[{"field":"title","code":"missing"}]}

== what the caller can see ==
    curl_do exit code: 1
    body file size:    0 bytes

VULNERABLE: the response body was discarded; only the status survived
```

## Why `-f` is there

It makes a failed request a failed command, which is what the retry loop and
every `||` in the library are written against. Simply dropping `-f` would make
`curl` return 0 for a 500 and quietly turn every error into a success — a much
worse bug than the one being fixed.

## Options

### A. Drop `-f`, branch on the status instead

Without `-f` the body is written for any status, and `%{http_code}` is reliable
because curl exits 0. `curl_do` then decides success from the status it already
parses, which is what its `case "${code}"` block at the end does anyway.

- The body is available for every status, including the ones worth reporting.
- The exit code becomes accurate: 4 for a 422 rather than 1.
- A transport failure (DNS, refused, timeout) still has to be told apart from an
  HTTP response, since curl exits non-zero only for the former. That is the
  distinction `-f` was blurring, so this is a clarification rather than a new
  problem.
- `-o` now writes a body the caller did not expect on failure. Anything that
  checks "did the file get written" as a proxy for success breaks — inside the
  library only `forge_release_upload` and the `ai` response path read the file
  after a failed call, and both already check the status first.

### B. Keep `-f`, add a second request on failure

Retry the failed request without `-f` to collect the body.

- No behaviour change on the success path.
- Doubles the request count on every error, which is exactly when an API is
  rate-limiting, and re-sends non-idempotent POSTs. Not acceptable.

### C. Keep `-f`, use `--write-out '%{stderr}...'` / `--fail-with-body`

`curl --fail-with-body` (7.76+, 2021) does precisely what is wanted: non-zero
exit *and* the body.

- One flag, no logic change.
- The library supports BusyBox and older distributions, where curl may predate
  7.76. It would need a version probe and a fallback to A anyway, so it becomes
  A plus a second code path.

## Recommendation

**A**, with the status-derived exit code, and `--fail-with-body` explicitly
rejected to avoid carrying two code paths for one behaviour.

The response body should be reachable rather than guessed at, so `forge_*`
messages become:

```sh
|| dybatpho::die "Could not create issue '${title}': $(dybatpho::forge_error)"
```

where `forge_error` reads `.message` out of the body the request already saved.

## What "done" looks like

- `test/security/http_error_body.sh` exits 0.
- `curl_do` returns 4 for a 422 and 5 for a 503, with the body present in both.
- A transport failure still returns 1 and is tested separately from an HTTP
  error.
- `forge_*` failures quote the API's own message; a test asserts a 422 with a
  `Validation Failed` body produces a message containing it.
- `doc/spec/network.md` and `doc/spec/forge.md` gain the requirements and `IT-`
  entries.
- `CHANGELOG.md` records under `### Fixed` that error bodies and error statuses
  were being discarded.
