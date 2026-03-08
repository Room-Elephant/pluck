```
██████╗ ██╗     ██╗   ██╗ ██████╗██╗  ██╗
██╔══██╗██║     ██║   ██║██╔════╝██║ ██╔╝
██████╔╝██║     ██║   ██║██║     █████╔╝
██╔═══╝ ██║     ██║   ██║██║     ██╔═██╗
██║     ███████╗╚██████╔╝╚██████╗██║  ██╗
╚═╝     ╚══════╝ ╚═════╝  ╚═════╝╚═╝  ╚═╝
```

> **Lightweight, label-based torrent media sorter.**

> Pluck watches your torrent client. Matches labels to rules. Moves files. Nothing more.

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow?style=flat-square)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-ghcr.io%2FRoom--Elephant%2Fpluck-2496ED?style=flat-square&logo=docker)](https://ghcr.io/Room-Elephant/pluck)

---

## Why pluck?

If you self-host a media server (Jellyfin, Plex, Audiobookshelf, Calibre-web) alongside a torrent client, you need a way to route completed downloads to the right library folder. The \*arr stack is powerful — but overkill if you just need **"label X → folder Y"**.

That's what pluck does. Nothing more, nothing less.

```
Like Sonarr's import, but without the rest of Sonarr.
```

---

## Quick Start

### 1. Create a rules file

Map torrent labels to destination folders. One rule per line: `label:destination`.

```ini
# rules.conf
audiobook:/data/media/audiobooks
ebook:/data/media/ebooks
music:/data/media/music
movie:/data/media/movies
tv:/data/media/tv
```

> Labels are matched **case-insensitively**. `Audiobook`, `AUDIOBOOK`, and `audiobook` all match the same rule.

### 2. Run with Docker Compose

```yaml
services:
  pluck:
    image: ghcr.io/Room-Elephant/pluck:latest
    restart: unless-stopped
    environment:
      - PLUCK_CLIENT=transmission
      - PLUCK_CLIENT_URL=http://transmission:9091
      - PLUCK_MODE=hardlink
      - PLUCK_WATCH_DIR=/data/downloads/complete
    volumes:
      - ./rules.conf:/config/rules.conf
      # Mount the common parent as a single volume (required for hardlink mode)
      - /data:/data
```

### 3. Label your torrents

Add labels in your torrent client. When a download completes, pluck picks it up and places it in the matching directory.

Torrents with **multiple labels** are processed per label — each label gets its own placement.

---

## How It Works

```
┌─────────────────┐   3. Read Label    ┌─────────┐   5. Place File    ┌──────────────────────┐
│  Torrent Client │◀───────────────────│         │───────────────────▶│ /media/audiobooks/   │
│    (labels)     │                    │  pluck  │                    │ /media/ebooks/       │
└─────────────────┘   2. File added    │         │                    │ /media/music/        │
         │          ┌─────────────────▶│         │                    └──────────────────────┘
         │          │                  └─────────┘
         │ 1. Done  │                       │
         ▼          │                       │ 4. Check Rules
┌─────────────────┐ │                       ▼
│  Watch Dir      │─┘                  rules.conf
│  /data/downloads│                   label:directory
└─────────────────┘
```

Pluck detects completed torrents two ways:

| Trigger | How |
|---|---|
| **Filesystem watcher** | Uses [`fsnotify`](https://github.com/fsnotify/fsnotify) to watch the downloads directory recursively. Fires after a **2-second debounce** to let files settle. |
| **Periodic rescan** | Catches torrents labeled after completion. Default: every 60 minutes, configurable via `PLUCK_RESCAN_INTERVAL`. |
---

## Configuration

All configuration is done via environment variables:

| Variable | Default | Description |
|---|---|---|
| `PLUCK_CLIENT` | `transmission` | Torrent client type |
| `PLUCK_CLIENT_URL` | `http://transmission:9091` | Client URL |
| `PLUCK_MODE` | `hardlink` | `hardlink`, `symlink`, or `copy` |
| `PLUCK_WATCH_DIR` | `/data/downloads` | Directory to watch for new files |
| `PLUCK_RULES_FILE` | `/config/rules.conf` | Path to rules file |
| `PLUCK_STATE_FILE` | `/config/state.txt` | Path to state history file |
| `PLUCK_RESCAN_INTERVAL` | `3600` | Seconds between periodic rescans |
| `PLUCK_DRY_RUN` | `false` | Log actions without executing them |
| `PLUCK_LOG_LEVEL` | `info` | `debug`, `info`, or `error` |

### Rules File Format

```ini
# One rule per line: label:destination
# Labels are matched case-insensitively
# Lines starting with # are comments

audiobook:/data/media/audiobooks
ebook:/data/media/ebooks
music:/data/media/music
movie:/data/media/movies
tv:/data/media/tv
```

### File Placement Modes

| Mode | Behavior | Cross-filesystem? | Disk usage |
|---|---|---|---|
| `hardlink` *(default)* | Hard-links files; source and destination share the same disk blocks | ❌ No | None |
| `symlink` | Creates a symbolic link pointing to the source file | ✅ Yes | None |
| `copy` | Copies files to the destination | ✅ Yes | Doubles usage |

> **Tip:** Use `hardlink` when downloads and media share a filesystem. Use `symlink` across different filesystems. Use `copy` when you need the most reliability, but it doubles disk usage.

> **⚠️ Hardlink mode:** Docker treats each bind-mount as a separate filesystem. To allow hardlinking between your downloads and media folders, mount their **common parent** (e.g., `/data:/data`) as a single volume. Mounting them individually will cause a `cross-device link` error.

> **⚠️ Symlink mode:** Ensure your media server container (Jellyfin, Plex, etc.) also mounts the downloads directory at the **same path**, so symlinks can resolve correctly.
---

## Supported Clients

| Client | Status |
|---|---|
| Transmission | ✅ Supported |
| qBittorrent | 🔜 Planned |
| Deluge | 🔜 Planned |

---

## Dry Run

Test your rules without touching any files:

```bash
docker run --rm \
  -e PLUCK_DRY_RUN=true \
  -e PLUCK_LOG_LEVEL=debug \
  -v ./rules.conf:/config/rules.conf \
  -v /data/downloads:/data/downloads \
  ghcr.io/Room-Elephant/pluck:latest
```

---

## Building from Source

**Docker:**
```bash
docker build -t pluck .
```

**Native (Go 1.26+):**
```bash
go build -o pluck ./cmd/pluck
```

---

## Roadmap

- [ ] qBittorrent support
- [ ] Deluge support
- [ ] Post-pluck webhook notifications (Audiobookshelf rescan, Discord, etc.)
- [ ] Post-pluck custom scripts
- [ ] Health check endpoint

---

## License

[MIT](LICENSE)