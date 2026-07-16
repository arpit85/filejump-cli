# FileJump CLI — User Guide

The FileJump CLI lets you manage your FileJump files and folders from the
terminal, and keep a folder on your computer in sync with your account.

This guide is for end users. You don't need to know Go or read the source code.

---

## 1. Install

The CLI is a single small program (`filejump`) with no dependencies. Download the
prebuilt binary for your operating system from the GitHub Releases page:

> https://github.com/arpit85/filejump-cli/releases

Pick the file that matches your system:

| Operating system | Download this file |
|---|---|
| macOS (Apple Silicon, M-series) | `filejump-darwin-arm64` |
| macOS (Intel) | `filejump-darwin-amd64` |
| Windows (64-bit, most PCs) | `filejump-windows-amd64.exe` |
| Windows (ARM) | `filejump-windows-arm64.exe` |
| Linux (most PCs) | `filejump-linux-amd64` |
| Linux (ARM, e.g. Raspberry Pi 4) | `filejump-linux-arm64` |

### macOS

1. Download `filejump-darwin-arm64` (or `amd64`).
2. Move it somewhere on your PATH, e.g.:
   ```bash
   mv ~/Downloads/filejump-darwin-arm64 /usr/local/bin/filejump
   chmod +x /usr/local/bin/filejump
   ```
3. The first time you run it, macOS may warn it's from an unidentified
   developer. To allow it: **System Settings → Privacy & Security → Open
   Anyway**. (Or run `xattr -d com.apple.quarantine /usr/local/bin/filejump`.)

### Windows

1. Download `filejump-windows-amd64.exe`.
2. Rename it to `filejump.exe` and move it to a permanent folder, e.g.
   `C:\Tools\filejump.exe`.
3. Add that folder to your `PATH` (search "Edit environment variables" in the
   Start menu), or run it from that folder as `.\filejump.exe`.

### Linux

```bash
mv ~/Downloads/filejump-linux-amd64 /usr/local/bin/filejump
chmod +x /usr/local/bin/filejump
```

### Check it works

```bash
filejump --help
```

You should see the list of commands.

---

## 2. Log in

Before anything else, log in once:

```bash
filejump login --server https://filejump.com
```

You'll be prompted for:

- **Email** — the email on your FileJump account
- **Password** — your FileJump password
- **Two-factor code** — only if you have 2FA enabled on your account

```
Server URL (e.g. https://filejump.com): https://filejump.com
Email: me@example.com
Password: ********
Logged in as Your Name (me@example.com)
```

Your login token is stored at `~/.config/filejump/config.json` (on Windows:
`%USERPROFILE%\.config\filejump\config.json`) with file mode `0600`. Your
password is **not** stored.

Check who you're logged in as:

```bash
filejump whoami
```

Log out (revokes the token and clears local config):

```bash
filejump logout
```

---

## 3. Paths

All paths use a leading `/`. Root is `/`.

- `/` — the top of your account
- `/Photos` — a folder called "Photos" at the top
- `/Photos/2026/trip.jpg` — a file inside nested folders

---

## 4. List your files

```bash
filejump ls                 # list root
filejump ls /Photos         # list a folder
filejump ls /Photos -l      # long listing (shows size, type, path)
```

Example output:

```
Photos/
Backups/
report.pdf
trip.jpg
```

With `-l`:

```
d  -        Photos  /Photos
d  -        Backups  /Backups
f  2.1MB    report.pdf
f  4.8MB    trip.jpg
```

(`d` = folder, `f` = file.)

---

## 5. Create folders

```bash
filejump mkdir /Backups              # create one folder
filejump mkdir /Backups/2026/jan -p   # create nested folders in one go
```

`-p` creates any missing parent folders along the way (like `mkdir -p`).

---

## 6. Upload files

```bash
filejump upload ./report.pdf /Backups          # upload into /Backups
filejump upload ./report.pdf /Backups/2026 -p  # create /Backups/2026 if missing
filejump upload ./photos /Photos/2026          # upload a whole folder tree
```

- The remote folder is optional; omit it to upload to your root: `filejump upload ./report.pdf`
- `-p` creates the remote folder if it doesn't exist.
- When you upload a **directory**, the CLI walks it and recreates the same
  structure remotely.

### Large files

Files **50 MB and larger** are uploaded directly to cloud storage using a
presigned URL — they don't pass through the web server, so they're faster and
more reliable. This happens automatically; you don't need any flag. If the
server can't do a direct upload, it falls back to normal upload automatically.

---

## 7. Download files

```bash
filejump download /Photos/2026/trip.jpg                # saves as trip.jpg here
filejump download /Photos/2026/trip.jpg ./saved.jpg    # save with a custom name
```

---

## 8. Move or rename files

```bash
filejump mv /Photos/old.jpg /Photos/new.jpg     # rename in place
filejump mv /Photos/old.jpg /Photos/2026/        # move into a folder (trailing slash)
filejump mv /Photos/old.jpg /Photos/2026         # also moves into /Photos/2026 if it's a folder
```

---

## 9. Delete files

```bash
filejump rm /Photos/old.jpg       # asks for confirmation
filejump rm /Photos/old.jpg -f    # skip the confirmation
```

Deleted files go to your FileJump trash (same as the web app), so you can
restore them from there if needed.

---

## 10. Workspaces

FileJump lets you keep some files in your **personal space** and others in a
**workspace** (a shared space you own or belong to, e.g. a team). The CLI
remembers which space is active and applies it to every data command — `ls`,
`upload`, `download`, `mkdir`, `mv`, `rm`, and `sync`.

### See your workspaces

```bash
filejump workspace ls
```

The active space is marked with `*`. Your personal space always appears as the
first row (`-` / `(personal)`).

### Switch to a workspace

```bash
filejump workspace use 5                 # by numeric id
filejump workspace use "Marketing Team"   # by name (quotes if it has spaces)
```

### Go back to your personal space

```bash
filejump workspace reset
# or equivalently:
filejump workspace use personal
filejump workspace use 0
```

### Check what's active

```bash
filejump workspace current
filejump whoami          # also shows the active workspace
```

### Run one command in a different space

You don't have to switch permanently. Add `--workspace <id>` (short form `-w`)
to any data command to override the active workspace for that single command:

```bash
filejump -w 5 ls /Campaigns
filejump -w 5 upload ./poster.pdf /Campaigns/2026
filejump -w 0 ls /Personal/Docs      # 0 forces the personal space
```

> Tip: `workspace ls` shows the ids and exact names you need for `use` and `-w`.

---

## 11. Two-way sync

`filejump sync` keeps a folder on your computer and a folder in your FileJump
account mirroring each other. Run it whenever you want to reconcile changes in
either direction.

```bash
filejump sync ./my-filejump /            # sync your whole account into ./my-filejump
filejump sync ./photos /Photos           # sync just /Photos
```

### What it does

Each time you run `filejump sync`, it does two things:

1. **Pull** — downloads new/changed files from FileJump to your computer,
   creates folders, and removes local copies of files that were deleted from
   your account.
2. **Push** — uploads new/changed files from your computer to FileJump
   (creating folders as needed), and deletes remote files you removed locally.

Sync remembers what it has already done using a small state file stored at
`~/.config/filejump/sync/`, so each run only transfers what changed. Each
local folder you sync keeps its own state, so you can sync multiple folders
independently.

### Recommended first run

The very first time you point sync at a folder, do a **pull-only** run so you
get a local copy without accidentally pushing anything:

```bash
filejump sync ./my-filejump / --no-push
```

After that, run normally to sync both ways:

```bash
filejump sync ./my-filejump /
```

### Conflicts

If a file changed on **both** your computer and FileJump since the last sync,
the CLI won't overwrite either copy. Instead it:

- keeps your local version renamed with a `.conflict` marker, e.g.
  `report.conflict.pdf`, and
- downloads the remote version as `report.pdf`.

You can then compare the two and keep what you want.

### One-directional sync

If you only want one direction:

```bash
filejump sync ./photos /Photos --no-push   # only pull from FileJump
filejump sync ./photos /Photos --no-pull   # only push to FileJump
```

### Important: sync can delete files

Two-way sync means **deleting a file locally and syncing will delete it from
your account too** (it goes to trash). If you're unsure, run with `--no-push`
first so nothing gets deleted remotely.

---

## 12. Troubleshooting

### "Not logged in. Run `filejump login` first."

Your token expired or was revoked. Just log in again:

```bash
filejump login --server https://filejump.com
```

### A large upload failed partway

Re-run the same `filejump upload` command. Uploads aren't resumable in v1, but
re-running will replace the file. For very large files, a stable network
connection helps.

### `sync` deleted a file I wanted to keep

Deleted files go to your FileJump trash. Restore them from the Trash in the
FileJump web app. To avoid this in future, use `--no-push` when you're not sure.

### macOS says the app "can't be opened because it's from an unidentified developer"

**System Settings → Privacy & Security → Open Anyway**. Or run:
```bash
xattr -d com.apple.quarantine /usr/local/bin/filejump
```

### I forgot which server I'm connected to

```bash
filejump whoami
```

### How do I start over?

```bash
filejump logout
filejump login --server https://filejump.com
```

---

## 13. Command reference

```text
filejump login [--server URL]            log in (stores a token)
filejump logout                          log out (revokes token, clears config)
filejump whoami                          show the logged-in account + active workspace

filejump ls [path] [-l]                   list folders/files
filejump mkdir <path> [-p]               create a folder (and parents with -p)
filejump upload <local> [remote] [-p]    upload a file or directory tree
filejump download <remote> [local]        download a file
filejump mv <src> <dest>                  move or rename a file
filejump rm <path> [-f]                   delete a file (confirms unless -f)
filejump sync <local-dir> [remote]        two-way sync a folder with FileJump

filejump workspace ls                    list workspaces you own or belong to
filejump workspace use <id|name>          switch the active workspace
filejump workspace current                show the active workspace
filejump workspace reset                  switch back to your personal space

Global flag (any data command):
  -w, --workspace <id>                   operate in this workspace (0 = personal)
```

Run `filejump <command> --help` for details on any command.

---

## Need more help?

- Releases and source: https://github.com/arpit85/filejump-cli
- Your files and trash are always available in the FileJump web app at
  https://filejump.com
