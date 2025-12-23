# Git Integration

Projekt supports automatic Git repository management through configuration. This feature allows you to:

- Define Git repositories in your configuration file
- Automatically clone missing repositories
- Check the status of configured repositories
- Verify remote URLs match configuration

## Configuration

### Git Servers

First, define your Git servers in `~/.config/projekt/config.yaml`:

```yaml
gitServers:
  - name: git3
    type: gitlab
    https: https://test.git.dev
    ssh: ssh://git@test.git.dev:2022
    preferGitSSH: false
  - name: github
    type: github
    https: https://github.com
    ssh: git@github.com
    preferGitSSH: true
```

### Folder Git Configuration

Then add Git configuration to your folders:

```yaml
folders:
  - path: /path/to/workspace
    prefix: "myproject"
    is_workspace: true
    git:
      host: git3 # Reference to gitServers name
      group: GROUP/SUBGROUP # Git group/namespace
      repos:
        - name: backend # Repository name
          path: api # Local folder name
        - name: frontend
          path: web
```

## Commands

### Check Repository Status

Check which repositories are missing or have issues:

```bash
projekt folder check
```

Output shows:
- `[OK]` - Repository exists and is valid
- `[MISSING]` - Repository doesn't exist locally
- `[NOT GIT]` - Directory exists but is not a Git repository
- `[WARNING]` - Repository exists but remote URL doesn't match

### Sync Repositories

Clone all missing repositories:

```bash
projekt folder sync
```

Use dry-run mode to preview changes:

```bash
projekt folder sync --dry-run
```

## Example Workflow

1. Define Git servers in your config file:

```yaml
gitServers:
  - name: github
    type: github
    https: https://github.com
    ssh: git@github.com
    preferGitSSH: true
```

2. Add Git configuration to your folders:

```yaml
folders:
  - path: /home/user/projects/myapp
    prefix: "app"
    is_workspace: true
    git:
      host: github # Reference to gitServers.name
      group: myorg/myteam
      repos:
        - name: backend
          path: backend
        - name: frontend
          path: frontend
        - name: mobile
          path: mobile-app
```

3. Check what needs to be cloned:

```bash
projekt folder check
```

4. Preview the sync operation:

```bash
projekt folder sync --dry-run
```

5. Sync repositories:

```bash
projekt folder sync
```

## Git Server Configuration

### SSH URL Formats

The tool supports two SSH URL formats:

1. **Standard format**: `git@host.com`
   - Example: `git@github.com`
   - Generates URLs like: `git@github.com:org/repo.git`

2. **SSH with port**: `ssh://git@host.com:port`
   - Example: `ssh://git@test.git.dev:2022`
   - Generates URLs like: `git@test.git.dev:2022/org/repo.git`

### HTTPS URLs

HTTPS URLs are used for validation when checking existing repositories. They follow the format:
- `https://host.com/group/repo.git`

### preferGitSSH Option

The `preferGitSSH` field controls which protocol to use for cloning:

**When `preferGitSSH: true`:**
- Primary: SSH URL
- Fallback: HTTPS URL (if SSH fails)
- Use this if you have SSH keys configured

**When `preferGitSSH: false`:**
- Primary: HTTPS URL only
- No fallback
- Use this if you prefer token-based authentication or don't have SSH keys

Example configurations:

```yaml
gitServers:
  # GitHub with SSH preference
  - name: github
    type: github
    https: https://github.com
    ssh: git@github.com
    preferGitSSH: true

  # GitLab with HTTPS preference
  - name: gitlab
    type: gitlab
    https: https://gitlab.com
    ssh: git@gitlab.com
    preferGitSSH: false
```

## Authentication

The Git integration uses SSH authentication. Make sure you have:

1. SSH keys configured for your Git host
2. Keys added to your SSH agent
3. Proper access permissions to the repositories

For SSH setup, see: [GitHub SSH Documentation](https://docs.github.com/en/authentication/connecting-to-github-with-ssh)

## Notes

- Repositories are cloned using SSH URLs
- Git server names must match between `gitServers` and folder `git.host`
- Empty `repos` array is valid (for future repositories)
- The `sync` command only clones missing repositories, it doesn't update existing ones
- Use `git pull` or your preferred Git workflow for updating existing repositories
- If a Git server is not found in configuration, an error will be displayed
