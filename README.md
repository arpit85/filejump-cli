# filejump CLI

A standalone command-line client for FileJump. Log in once and manage your files
and folders from the terminal.

The CLI talks to the existing Laravel Sanctum REST API (`/api/*`) — no special
server-side setup is required beyond a normal FileJump account.

**Repository:** https://github.com/arpit85/filejump-cli

## Install

### Prebuilt binary (recommended)

Download the latest release for your OS from the
[Releases page](https://github.com/arpit85/filejump-cli/releases), then put the
`filejump` binary on your `PATH`. See [`GUIDE.md`](GUIDE.md) for per-OS steps.

### From source

```bash
git clone https://github.com/arpit85/filejump-cli.git
cd filejump-cli
make build
# optionally:
make install   # copies `filejump` to your GOPATH/bin
```

### Build release binaries

```bash
make release   # builds dist/filejump-<os>-<arch> for common platforms
```

## Getting started

```bash
filejump login --server https://filejump.com
# prompts for email + password (and 2FA code if enabled)
```

Your API token and server URL are stored at `~/.config/filejump/config.json`
(mode `0600`).

## Commands

```text
filejump login [--server URL]            authenticate and store a token
filejump logout                          revoke token and clear local config
filejump whoami                          show the logged-in account

filejump ls [path] [-l]                   list folders/files (default: root)
filejump mkdir <path> [-p]               create a folder (and parents with -p)
filejump upload <local> [remote] [-p]    upload a file or directory tree
filejump download <remote> [local]        download a file
filejump mv <src> <dest>                  move or rename a file
filejump rm <path> [-f]                   delete a file (confirms unless -f)
filejump sync <local-dir> [remote]        two-way sync a local dir with a remote folder

filejump workspace ls                    list workspaces you own or belong to
filejump workspace use <id|name>          switch the active workspace
filejump workspace current                show the active workspace
filejump workspace reset                  switch back to your personal space
```

Paths are written as `/Photos/2026/trip.jpg`. Root is `/`.

Any data command also accepts `--workspace <id>` (or `-w <id>`) to operate in a
specific workspace for that single invocation, overriding the saved active
workspace. Use `-w 0` to force the personal space.

### Examples

```bash
# List root
filejump ls

# Long listing of a folder
filejump ls /Photos/2026 -l

# Create a nested folder tree
filejump mkdir /Backups/2026/jan -p

# Upload a single file into a folder (create folder if missing)
filejump upload ./report.pdf /Backups/2026 -p

# Upload a whole directory tree (preserves structure)
filejump upload ./photos /Photos/2026

# Download a file
filejump download /Photos/2026/DSC_0001.JPG ./DSC_0001.JPG

# Rename a file in place
filejump mv /Photos/2026/old.jpg /Photos/2026/new.jpg

# Move a file into another folder
filejump mv /Photos/2026/old.jpg /Photos/2027/

# Delete a file
filejump rm /Photos/2026/old.jpg
```

## Uploads: multipart vs. presigned S3

Files **under 50 MB** use the simple multipart endpoint (`POST /api/files/upload`).
Files **50 MB and above** use the presigned S3 flow:

1. `POST /api/files/upload-url` → get a presigned PUT URL + storage path,
2. `PUT` the file body directly to S3 (bypassing the app server), then
3. `POST /api/files/upload-complete` to register the file in the database.

If the server has no S3 storage available it returns `FALLBACK_TO_PROXY`, and the
CLI transparently falls back to the multipart endpoint. No flags needed — the
strategy is chosen automatically per file.

## Two-way sync

`filejump sync <local-dir> [remote-root]` keeps a local directory and a remote
folder in both directions.

```bash
# Mirror your entire account into ./my-filejump
filejump sync ./my-filejump /

# Sync only /Photos into ./photos
filejump sync ./photos /Photos
```

How it works:

- **Pull** — consumes the incremental `/api/sync/delta` feed (cursor-based) and
  downloads new/changed remote files, creates folders, and removes local files
  that were deleted remotely.
- **Push** — walks the local tree, uploads new/modified files (auto-creating
  remote folders), and deletes remote files that were removed locally.
- **Conflicts** — if a file changed on both sides since the last sync, the local
  copy is preserved as `name.conflict.ext` and the remote version is downloaded.

Sync state (cursor + per-file metadata) is stored per local directory at
`~/.config/filejump/sync/<hash>.json`, so multiple trees can sync independently.

Flags:

- `--no-pull` — only push local changes.
- `--no-push` — only pull remote changes.

> Note: two-way sync is powerful — deleting files locally and syncing will
> delete them remotely too. Use `--no-push` to inspect remote changes safely.

## Workspaces

FileJump scopes every file and folder under either your **personal space** or a
**workspace** you own or belong to. The CLI remembers which one is active and
applies it to all data commands (`ls`, `upload`, `download`, `mkdir`, `mv`,
`rm`, `sync`).

```bash
# List workspaces (the active one is marked with *)
filejump workspace ls

# Switch by id or by name
filejump workspace use 5
filejump workspace use "Marketing Team"

# Go back to your personal space
filejump workspace reset        # or: filejump workspace use personal

# Show what's active
filejump workspace current
filejump whoami                 # also prints the active workspace
```

For a one-off command in a different workspace without switching, pass
`--workspace <id>` (or `-w <id>`):

```bash
filejump -w 5 ls /Campaigns
filejump -w 0 upload ./logo.png /Branding     # 0 = personal space
```

## Notes & limitations

- Share-link management and token management are planned for a later release.
- On `401 Unauthorized`, re-run `filejump login`.

## Development

```bash
make vet     # go vet
make test    # go test
make run ARGS=ls
```

## End-user guide

For a non-technical walkthrough (install, login, every command, sync, and
troubleshooting), see [`GUIDE.md`](GUIDE.md).
