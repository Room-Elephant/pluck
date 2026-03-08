# 🍒 pluck

**Lightweight, label-based torrent media sorter.**

Pluck watches your torrent client for completed downloads, matches their labels against your rules, and automatically hardlinks (or symlinks/copies) them to the right media directories.

> *Like Sonarr's import, but without the rest of Sonarr.*

## Why?

If you self-host a media server (Jellyfin, Plex, Audiobookshelf, Calibre-web) alongside a torrent client, you need a way to route completed downloads to the right library folder. The \*arr stack is powerful but overkill if you just need **"label X → folder Y"**. That's what pluck does — nothing more, nothing less.

## Quick Start

### 1. Create a rules file

```
# rules
audiobook:/data/media/audiobooks
ebook:/data/media/ebooks
music:/data/media/music
```

### 2. Run with Docker Compose

```yaml
services:
  pluck:
    image: ghcr.io/jpedroborges/pluck:latest
    container_name: pluck
    restart: unless-stopped
    environment:
      - PLUCK_CLIENT=transmission
      - PLUCK_CLIENT_URL=http://transmission:9091/transmission/rpc
      - PLUCK_MODE=hardlink
      - PLUCK_WATCH_DIR=/data/downloads
    volumes:
      - ./rules:/etc/pluck/rules
      - /data/downloads:/data/downloads
      - /data/media:/data/media
```

### 3. Label your torrents

Add labels in your torrent client (e.g., `audiobook`, `ebook`, `music`). When a torrent completes, pluck picks it up and places it in the matching directory.

## How It Works

```
┌──────────────┐     ┌───────┐     ┌─────────────────────┐
│ Torrent      │     │       │     │ /media/audiobooks/   │
│ Client       │────▶│ pluck │────▶│ /media/ebooks/       │
│ (labels)     │     │       │     │ /media/music/        │
└──────────────┘     └───────┘     └─────────────────────┘
                         │
                    Rules File
                  label:directory
```

Pluck detects completed torrents in two ways:
1. **Filesystem watcher** (`inotifywait`) — reacts immediately when files appear in the downloads directory
2. **Periodic rescan** — catches torrents that were labeled after completion (default: every 60 minutes)

## Configuration

All configuration is done via environment variables:

| Variable | Default | Description |
|---|---|---|
| `PLUCK_CLIENT` | `transmission` | Torrent client type |
| `PLUCK_CLIENT_URL` | `http://transmission:9091/transmission/rpc` | Client RPC endpoint |
| `PLUCK_CLIENT_USER` | *(empty)* | RPC auth username |
| `PLUCK_CLIENT_PASS` | *(empty)* | RPC auth password |
| `PLUCK_MODE` | `hardlink` | `hardlink`, `symlink`, or `copy` |
| `PLUCK_WATCH_DIR` | `/data/downloads` | Directory to watch for new files |
| `PLUCK_RULES_FILE` | `/etc/pluck/rules` | Path to label→directory rules file |
| `PLUCK_RESCAN_INTERVAL` | `3600` | Seconds between periodic rescans |
| `PLUCK_DRY_RUN` | `false` | Log actions without executing them |
| `PLUCK_LOG_LEVEL` | `info` | `debug`, `info`, or `error` |

### Rules File Format

```
# One rule per line: label:destination
# Labels are matched case-insensitively
# Lines starting with # are comments

audiobook:/data/media/audiobooks
ebook:/data/media/ebooks
music:/data/media/music
movie:/data/media/movies
tv:/data/media/tv
```

### File Modes

| Mode | Behavior | Cross-filesystem? | Disk usage |
|---|---|---|---|
| `hardlink` (default) | Creates hard links; files share the same disk blocks | ❌ No | No extra space |
| `symlink` | Creates symbolic links pointing to the source | ✅ Yes | No extra space |
| `copy` | Copies files to the destination | ✅ Yes | Doubles disk usage |

> **Tip:** Use `hardlink` when downloads and media are on the same filesystem. Use `symlink` or `copy` when they're on different filesystems.

## Supported Clients

| Client | Status |
|---|---|
| Transmission | ✅ Supported |
| qBittorrent | 🔜 Planned |
| Deluge | 🔜 Planned |

## Building from Source

```bash
docker build -t pluck .
```

## Dry Run

Test your rules without moving any files:

```bash
docker run --rm \
  -e PLUCK_DRY_RUN=true \
  -e PLUCK_LOG_LEVEL=debug \
  -v ./rules:/etc/pluck/rules \
  -v /data/downloads:/data/downloads \
  pluck
```

## Roadmap

- [ ] qBittorrent support
- [ ] Deluge support
- [ ] Post-pluck webhook notifications (Audiobookshelf rescan, Discord, etc.)
- [ ] Post-pluck custom scripts
- [ ] GHCR / Docker Hub automated builds
- [ ] Health check endpoint

## License

[MIT](LICENSE)
