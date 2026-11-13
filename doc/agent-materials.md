# AI materials

Coding agents read what a project hands them: an `AGENTS.md`, a few skills, a
note about where you stopped. Those are files, so they are templates like any
other, and `t` keeps them the same across every project you make.

## Which file is for what

**`AGENTS.md`** holds what is true in every session: what the project is, the
commands that build and test it, the paths nobody edits. It is read every time,
so every line costs attention. One that grows into a manual nobody reads
teaches the agent to skim it; keep what an agent would get wrong without it.

**A skill** is a procedure needed sometimes: cutting a release, adding a
migration. It is loaded only when its description matches the request. A
procedure stuffed into `AGENTS.md` is paid for in every session; a skill is
paid for only when it is used.

**The project note** (`projekt folder note`) is what is half-done right now.
It changes by the day, so it does not belong in a committed file, where it goes
stale and then misleads.

```bash
projekt folder note -a "migration half done, see PR 42"
projekt folder note
```

## Writing a skill an agent actually uses

The `description` in the front matter is the trigger. An agent decides from it
alone whether to open the skill, so write when to use it, in the words the
request will use: "Use when the user asks to cut a release or tag a version",
not "Release management". A skill that never triggers is a skill you did not
write.

The body is steps, each with its reason: "Run the tests first; a release of a
red build cannot be pulled back" gets followed, "Run the tests" gets skipped
when it looks slow. Keep one skill to one job; two jobs mean a description that
matches neither well.

`agent-skill` scaffolds one: it asks for the description and takes the name from
the folder, as agents look a skill up by folder. It writes the front matter and
`When to use`, `Steps` and `Checks` headings for you to fill in.

```bash
t new agent-skill ./release -i
```

## One source for every agent

Agents look for different file names. `agent-docs` writes `AGENTS.md` and makes
`CLAUDE.md` and `GEMINI.md` links to it, so there is one file to keep true:

```bash
t new agent-docs . -i
```

`-i` asks for the build, test and lint commands, the conventions and the
off-limits paths. The project's name and description come from what projekt
knows about it ([below](#what-the-template-knows)). The answers are recorded in
`.projekt/`, so the next two commands need no flags:

```bash
t diff        # what applying would change; "3 unchanged" when nothing
t apply       # write it
```

Run both inside the project. Change a template, or one answer with
`t apply --set test='make check'`, and the change reaches every project made
from it.

Edits you made by hand survive. `t apply` merges three ways: the file as it was
written, as it is now, and as the template renders it today. A region only one
side changed takes that side. Where you and the template changed the same lines
differently, the file is kept as it is and `t diff` says so, exiting non-zero:

```
conflict	AGENTS.md  (edited since, kept — use --force)
1 conflict, 2 unchanged
```

Nothing is written with conflict markers in it. Settle it in the file, or
`--force` to take the template's version. [Templates](templates.md#applying-a-template-again)
has the rest.

## Starting from an AGENTS.md you already have

`t new agent-docs .` refuses a file that is there, and it works file by file:
`AGENTS.md` may be written before `CLAUDE.md` is refused. Run it with the
project's work committed, so that nothing is lost either way.

```bash
git add -- $(ls AGENTS.md CLAUDE.md GEMINI.md 2>/dev/null)
git commit -m "before agent-docs"
```

Only when the commit succeeded — or git says there is nothing to commit,
because the files are already in git — render over them:

```bash
t new agent-docs . -i --force
git diff
```

`--force` overwrites, and `git diff` shows what your hand-written file had that
the template does not. Put those parts back with an editor, then commit. From
then on `t apply` merges instead of overwriting.

## What the template knows

Every template gets `.Folder`: what projekt has for the project the files go
into. Its five fields are `.Folder.Name`, `.Path`, `.Tags` (a list),
`.Description` and `.Workspace`. `agent-docs` opens with it:

```
# {{ .Folder.Name | default (base .Dir) }}
```

For a new project directly under a configured workspace the fields are already
filled in. Outside a configured project they are empty, which is why the line
above falls back to the folder's name.

## Reaching every agent

projekt writes files in the project. Getting a skill or a rule into the agents
themselves is the job of an agent manager such as dyai.
