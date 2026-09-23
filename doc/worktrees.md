# Working trees

Reviewing a pull request while your own branch is half-finished is the moment
`git stash` was invented for, and the moment it is worst at. A working tree is
another checkout of the same repository, on another branch, with its own
files — nothing to stash, nothing to rebuild.

`projekt worktree` creates them and keeps track of them, so they are reachable
by name like any other project:

```bash
projekt worktree add myapp feature/PROJ-123
pj myapp@PROJ-123
```

## The name

The part after the `@` comes from the last element of the branch:
`feature/PROJ-123` becomes `PROJ-123`, because the ticket is what tells two
branches apart, and it is what you have to type. `--name` overrides it.

Anything that is not a letter, a digit, `.`, `_` or `-` becomes a `-`, so a
branch name never turns into a folder name that has to be quoted.

## Where they live

By default inside the project, in `<project>/.worktrees/<name>`, which keeps a
working tree beside the code it belongs to. Git does not ignore that folder on
its own; this keeps it out of `git status` without touching a tracked file:

```bash
echo '.worktrees/' >> .git/info/exclude
```

`--path` puts it somewhere else, absolute or relative to the project. Putting
one directly inside a workspace is worth avoiding — the workspace picks it up
as a project of its own, so the same folder ends up answering to two names.
`projekt worktree add` says so when it happens.

## Nothing else had to learn about them

A working tree resolves through the same short names as everything else, so
these all work with no extra machinery:

```bash
pj myapp@PROJ-123                 # jump to it
projekt folder get myapp@PROJ-123 # what pj calls
projekt folder list -t work       # it carries its project's tags
```

Completion included: `pj <TAB>` offers working trees because it offers short
names, and a working tree has one. The shell integration was not changed.

## Creating one

```bash
projekt worktree add myapp feature/PROJ-123   # then pj myapp@PROJ-123
projekt worktree add myapp release/1.0 --name trunk
projekt worktree add myapp hotfix --path ../myapp-hotfix
projekt worktree add myapp feature/x --dry-run
```

The branch is checked out when it exists and created from the current HEAD when
it does not, so starting a branch and picking one up again are the same command.

Git refuses to check out a branch that is already checked out somewhere else,
including in the project itself — which is why `--name trunk` for `main` only
works if the project is not sitting on `main`.

## Seeing what you have

```bash
projekt worktree list
projekt worktree list myapp          # only one project's
projekt worktree list -t work        # by the project's tags
projekt worktree list -o json
projekt worktree list --no-status    # skip asking git, one call per project
```

The status column compares the configuration with the repository:

| Status | Meaning |
| ------ | ------- |
| `ok` | It is there, on the branch it says |
| `MISSING` | The folder is gone. `remove --keep` drops the entry |
| `NOT A WORKTREE` | The folder is there, but git does not count it as one of this repository's |
| `BRANCH DRIFTED` | It is checked out on another branch than the one recorded |
| `NO PROJECT` | The project it hangs off no longer resolves |

`projekt config check` reports that last one as an error, because a name that
leads nowhere is worse than no name.

## Putting one away

```bash
projekt worktree remove myapp@PROJ-123
projekt worktree remove myapp@PROJ-123 --force  # it has changes in it
projekt worktree remove myapp@PROJ-123 --keep   # only drop the entry
```

**The branch is left alone.** Putting away the folder it was checked out in and
deleting the work are two different decisions, and only one of them is this
command's. Delete the branch with `git branch -d` when you mean to.

## In the configuration

```yaml
worktrees:
  - project: oss-mytool          # the short name, as pj knows it
    name: PROJ-123               # the part after the @
    branch: feature/PROJ-123
    path: /home/me/oss/mytool/.worktrees/PROJ-123
```

They hang off the project's **short name** rather than off a folder entry,
because a project is often a child of a workspace and has no entry of its own.
Renaming a project therefore orphans its working trees; `projekt config check`
says which.

> Note: the `worktrees:` under a repository in a folder's `git:` section is a
> different thing — those are created by `projekt folder sync` from a
> configuration you write by hand, for a checkout you always want. These are
> the ones you make during the day and throw away after.
