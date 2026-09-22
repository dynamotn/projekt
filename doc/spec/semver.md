# Feature Specification: Semantic Version Utilities

**Feature Branch**: `[reverse-spec-semver]`
**Status**: Implemented
**Input**: Existing source analysis: `src/semver.sh`, `doc/semver.md`, `test/semver.bats`, and `example/semver_ops.sh`

## Problem Statement *(mandatory)*

Release scripts need to validate, inspect, compare, and increment
Semantic-Version-like values. Reimplementing precedence rules and version
parsing in Bash is error-prone, especially for pre-release and build metadata.
This module follows SemVer 2.0.0 precedence while accepting the syntax defined
by its current validation regex.

## Business Value *(mandatory)*

- Centralize SemVer 2.0.0 behavior for release automation.
- Make version comparisons and bumps predictable across scripts.
- Provide structured components for changelog and artifact workflows.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Validate and parse versions (Priority: P1)

As a release script author, I want to validate SemVer strings and extract
their components so that invalid releases fail early and metadata is reusable.

**Independent Test**: Validate canonical, `v`-prefixed, pre-release, build,
and invalid strings, then inspect parsed components.

**Acceptance Scenarios**:

1. **Given** a valid SemVer 2.0.0 string with an optional leading `v`, **When**
   validation runs, **Then** it succeeds
2. **Given** an invalid version, **When** validation or parsing runs, **Then**
   it fails with no misleading parsed result
3. **Given** a valid version, **When** parsing runs, **Then** major, minor,
   patch, pre-release, and build metadata are printed on separate lines

### User Story 2 - Compare versions correctly (Priority: P1)

As a maintainer, I want precedence-aware comparison so that release gates and
dependency checks handle numeric and alphanumeric pre-release identifiers.

**Independent Test**: Compare numeric core changes, release vs pre-release,
numeric vs alphanumeric identifiers, and equal versions with differing build
metadata.

**Acceptance Scenarios**:

1. **Given** two valid versions, **When** comparison runs, **Then** it prints
   `-1`, `0`, or `1` according to SemVer precedence
2. **Given** versions differ only in build metadata, **When** comparison runs,
   **Then** it reports equality because build metadata does not affect
   precedence

### User Story 3 - Bump and classify releases (Priority: P1)

As a release engineer, I want to bump major/minor/patch values and classify
the change between two versions so that release automation can select the
right workflow.

**Independent Test**: Bump each supported part with optional metadata and
classify core, pre-release, build-only, and equal changes.

**Acceptance Scenarios**:

1. **Given** a valid version and `major`, `minor`, or `patch`, **When** bump
   runs, **Then** the selected component increments and lower components reset
   as required
2. **Given** optional pre-release or build values, **When** bump runs, **Then**
   those values are attached to the new version
3. **Given** two valid versions, **When** release type runs, **Then** it prints
   `major`, `minor`, `patch`, `pre-release`, `build`, or `equal`

### Example Workflow

```bash
current="v1.4.2"
dybatpho::semver_valid "${current}" || dybatpho::die "Not a semantic version"

next="$(dybatpho::semver_bump "${current}" minor)"          # 1.5.0
candidate="$(dybatpho::semver_bump "${current}" minor rc.1)" # 1.5.0-rc.1

dybatpho::info "release type: $(dybatpho::semver_release_type "${current}" "${next}")"

if dybatpho::semver_compare "${next}" "${current}"; then
  dybatpho::info "${next} supersedes ${current}"
fi
```

## Edge Cases

- A version has a leading `v`.
- Numeric components or metadata use syntax accepted by the current validation
  regex, including forms that a stricter SemVer parser might reject.
- Pre-release identifiers are mixed numeric and alphanumeric segments.
- Build metadata differs while precedence remains equal.
- A version is missing, malformed, or has invalid components.
- An unsupported bump part is supplied.
- Bumping drops source pre-release/build metadata unless replacements are given.

### User Story - Check a version against a range (Priority: P1)

As a script author, I want to ask whether a version satisfies a range so that a dependency check reads as the requirement itself rather than as a chain of comparisons.

**Why this priority**: Comparing two versions is already possible; expressing "at least 18" or "compatible with 1.2" is what a real check needs, and hand-rolling it from comparisons is where the mistakes are.

**Independent Test**: Test versions against caret, tilde, comparison, wildcard, and alternative ranges, and verify the pre-release rule.

**Acceptance Scenarios**:

1. **Given** a caret range, **When** a version is tested, **Then** only changes that leave the leftmost non-zero part alone satisfy it
2. **Given** a tilde range, **When** a version is tested, **Then** only patch-level changes satisfy it
3. **Given** several comparators separated by spaces, **When** a version is tested, **Then** all of them must hold
4. **Given** alternatives separated by `||`, **When** a version is tested, **Then** satisfying any one of them is enough
5. **Given** a pre-release version, **When** it is tested against a range that names no pre-release of the same release, **Then** it does not satisfy the range

---

### User Story - Order a list of versions (Priority: P2)

As a release script, I want a list of versions ordered as versions so that the newest tag is the newest release rather than the last one alphabetically.

**Why this priority**: Without it every caller reimplements the ordering, usually with `sort` and usually wrong for pre-releases.

**Independent Test**: Sort a list whose string order differs from its version order, including pre-releases, and take the maximum.

**Acceptance Scenarios**:

1. **Given** versions whose string order differs from their version order, **When** they are sorted, **Then** the version order wins
2. **Given** a pre-release and the release it precedes, **When** they are sorted, **Then** the pre-release comes first
3. **Given** a list on standard input, **When** it is sorted, **Then** any leading `v` is preserved so a list of tags stays usable

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-R01**: The module MUST report whether a version satisfies a range written with carets, tildes, comparison operators, partial versions, wildcards, space-separated comparators meaning all of them, and `||` meaning any of them.
- **FR-R02**: A caret range MUST allow only changes that leave the leftmost non-zero part unchanged, so that `^0.2.3` stops at `0.3.0` and `^0.0.3` stops at `0.0.4`.
- **FR-R03**: A tilde range MUST allow patch-level changes, and MUST widen to minor-level when only the major was named.
- **FR-R04**: A pre-release MUST satisfy a range only when the range names a pre-release of the same `major.minor.patch`, so that a range below the next major does not quietly accept its pre-release.
- **FR-R05**: A range that cannot be understood MUST stop the caller rather than reporting a result, and MUST NOT be validated inside a command substitution, where the rejection could not reach the caller.
- **FR-R06**: The module MUST order versions by the specification's rules, accepting the list as arguments or on standard input, preserving a leading `v`, and MUST report the highest of them.

- **FR-001**: The module MUST validate versions against its supported SemVer
  grammar with an optional leading `v`.
- **FR-002**: `semver_parse` MUST print major, minor, patch, pre-release, and
  build metadata as five lines.
- **FR-003**: Invalid versions MUST fail validation and parsing clearly.
- **FR-004**: `semver_compare` MUST implement SemVer 2.0.0 precedence,
  including numeric core and pre-release identifier ordering.
- **FR-005**: Build metadata MUST be ignored by precedence comparison.
- **FR-006**: `semver_bump` MUST support only `major`, `minor`, and `patch`.
- **FR-007**: Major bumps MUST reset minor and patch; minor bumps MUST reset
  patch; patch bumps MUST preserve major and minor.
- **FR-008**: Bumps MUST remove source pre-release/build metadata and optionally
  append caller-provided replacements.
- **FR-009**: `semver_release_type` MUST classify major, minor, patch,
  pre-release, build-only, and equal changes.
- **FR-010**: Release classification MUST reject invalid input versions.
- **FR-011**: The module MUST expose `DYBATPHO_SEMVER_REGEX` for the active
  validation grammar.

### Key Entities *(include if feature involves data)*

- **SemVer**: A `major.minor.patch` version with optional pre-release and build
  metadata.
- **Pre-release Identifier**: Dot-separated numeric or alphanumeric identifier
  compared with SemVer precedence rules.
- **Build Metadata**: Optional `+` metadata ignored for precedence but
  distinguishable by release classification.
- **Release Type**: The category emitted for a version transition.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Release scripts can validate and compare versions without custom
  parsing logic.
- **SC-002**: Version bumps preserve SemVer reset rules and requested metadata.
- **SC-003**: Release classification distinguishes precedence changes,
  pre-release changes, build-only changes, and equality.

## Integration Tests *(mandatory)*

- **IT-R01**: Test caret ranges above and below 1.0, tilde ranges at each specificity, plain comparisons, partial versions, and wildcards.
- **IT-R02**: Test several comparators together, and alternatives separated by `||`.
- **IT-R03**: Verify a pre-release is refused by a range that never named one, and accepted by a range that names one of the same release.
- **IT-R04**: Verify an invalid version and an invalid range both stop the caller.
- **IT-R05**: Sort lists whose string order differs from their version order, including pre-releases, from arguments and from standard input, and verify the maximum.
- **IT-R06**: Verify the sort agrees with the comparison helper for every adjacent pair of its result.

- **IT-001**: Validate canonical, `v`-prefixed, pre-release, build, and invalid
  versions.
- **IT-002**: Parse all five SemVer components and verify empty optional lines.
- **IT-003**: Compare core and pre-release precedence plus build-only changes.
- **IT-004**: Bump major, minor, and patch versions with optional metadata.
- **IT-005**: Classify major, minor, patch, pre-release, build, and equal pairs.
- **IT-006**: Reject invalid versions and unsupported bump parts.

## Acceptance Criteria *(mandatory)*

1. SemVer behavior follows the documented SemVer 2.0.0 precedence contract.
2. Output helpers are suitable for command substitution and release scripts.
3. Invalid inputs fail rather than producing a partially interpreted version.
