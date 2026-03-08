#!/bin/sh
# pluck — lightweight, label-based torrent media sorter
# https://github.com/jpedroborges/pluck

set -eu

# ---------------------------------------------------------------------------
# Configuration (all overridable via environment variables)
# ---------------------------------------------------------------------------
PLUCK_CLIENT="${PLUCK_CLIENT:-transmission}"
PLUCK_CLIENT_URL="${PLUCK_CLIENT_URL:-http://transmission:9091/transmission/rpc}"
PLUCK_CLIENT_USER="${PLUCK_CLIENT_USER:-}"
PLUCK_CLIENT_PASS="${PLUCK_CLIENT_PASS:-}"
PLUCK_MODE="${PLUCK_MODE:-hardlink}"
PLUCK_WATCH_DIR="${PLUCK_WATCH_DIR:-/data/downloads}"
PLUCK_RULES_FILE="${PLUCK_RULES_FILE:-/etc/pluck/rules}"
PLUCK_RESCAN_INTERVAL="${PLUCK_RESCAN_INTERVAL:-3600}"
PLUCK_DRY_RUN="${PLUCK_DRY_RUN:-false}"
PLUCK_LOG_LEVEL="${PLUCK_LOG_LEVEL:-info}"

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
_log_level_value() {
  case "$1" in
    debug) echo 0 ;;
    info)  echo 1 ;;
    error) echo 2 ;;
    *)     echo 1 ;;
  esac
}

CURRENT_LOG_LEVEL=$(_log_level_value "$PLUCK_LOG_LEVEL")

log_debug() { [ "$CURRENT_LOG_LEVEL" -le 0 ] && echo "[pluck:debug] $*" || true; }
log_info()  { [ "$CURRENT_LOG_LEVEL" -le 1 ] && echo "[pluck] $*"       || true; }
log_error() { echo "[pluck:error] $*" >&2; }

# ---------------------------------------------------------------------------
# Rules loading
# ---------------------------------------------------------------------------
# Rules are stored as newline-separated "label:directory" pairs
RULES=""

load_rules() {
  if [ ! -f "$PLUCK_RULES_FILE" ]; then
    log_error "Rules file not found: $PLUCK_RULES_FILE"
    exit 1
  fi

  RULES=""
  while IFS= read -r line || [ -n "$line" ]; do
    # skip empty lines and comments
    case "$line" in
      ""|\#*) continue ;;
    esac

    label=$(echo "$line" | cut -d: -f1 | tr '[:upper:]' '[:lower:]' | xargs)
    directory=$(echo "$line" | cut -d: -f2- | xargs)

    if [ -z "$label" ] || [ -z "$directory" ]; then
      log_error "Invalid rule (skipping): $line"
      continue
    fi

    RULES="${RULES}${label}:${directory}
"
    log_debug "Rule loaded: '$label' -> '$directory'"
  done < "$PLUCK_RULES_FILE"

  rule_count=$(echo "$RULES" | grep -c ':' || true)
  log_info "Loaded $rule_count rule(s) from $PLUCK_RULES_FILE"
}

get_destination_for_label() {
  torrent_label=$(echo "$1" | tr '[:upper:]' '[:lower:]')
  echo "$RULES" | while IFS= read -r rule; do
    [ -z "$rule" ] && continue
    rule_label=$(echo "$rule" | cut -d: -f1)
    rule_dir=$(echo "$rule" | cut -d: -f2-)
    if [ "$torrent_label" = "$rule_label" ]; then
      echo "$rule_dir"
      return
    fi
  done
}

# ---------------------------------------------------------------------------
# File operations
# ---------------------------------------------------------------------------
place_file() {
  src="$1"
  dst="$2"

  if [ "$PLUCK_DRY_RUN" = "true" ]; then
    log_info "[dry-run] Would $PLUCK_MODE: $(basename "$src") -> $dst"
    return 0
  fi

  # Ensure destination parent directory exists
  mkdir -p "$(dirname "$dst")"

  case "$PLUCK_MODE" in
    hardlink)
      if [ -d "$src" ]; then
        cp -rl "$src" "$dst"
      else
        ln "$src" "$dst"
      fi
      ;;
    symlink)
      ln -sf "$src" "$dst"
      ;;
    copy)
      cp -r "$src" "$dst"
      ;;
    *)
      log_error "Unknown mode: $PLUCK_MODE"
      return 1
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Torrent client: Transmission
# ---------------------------------------------------------------------------
transmission_auth_header() {
  if [ -n "$PLUCK_CLIENT_USER" ] && [ -n "$PLUCK_CLIENT_PASS" ]; then
    echo "-u ${PLUCK_CLIENT_USER}:${PLUCK_CLIENT_PASS}"
  fi
}

transmission_get_session_id() {
  # shellcheck disable=SC2046
  curl -s --max-time 5 $(transmission_auth_header) \
    "$PLUCK_CLIENT_URL" -D - -o /dev/null 2>/dev/null | \
    grep -i 'X-Transmission-Session-Id:' | head -1 | awk '{print $2}' | tr -d '\r\n'
}

transmission_get_completed_torrents() {
  SESSION_ID=$(transmission_get_session_id)

  if [ -z "$SESSION_ID" ]; then
    log_debug "Could not get Transmission session ID, skipping..."
    return 0
  fi

  # shellcheck disable=SC2046
  RESPONSE=$(curl -s --max-time 10 \
    $(transmission_auth_header) \
    -H "X-Transmission-Session-Id: $SESSION_ID" \
    -d '{"method":"torrent-get","arguments":{"fields":["name","downloadDir","labels","percentDone"]}}' \
    "$PLUCK_CLIENT_URL" 2>/dev/null)

  if [ -z "$RESPONSE" ]; then
    log_debug "Empty response from Transmission, skipping..."
    return 0
  fi

  # Output format: label|source_path (one per line, per label)
  # A torrent can have multiple labels — we emit a line for each matching label
  echo "$RESPONSE" | jq -r '
    .arguments.torrents[]
    | select(.percentDone == 1)
    | . as $t
    | .labels[]
    | ascii_downcase as $label
    | ($label + "|" + $t.downloadDir + "/" + $t.name)
  ' 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Client dispatcher
# ---------------------------------------------------------------------------
get_completed_torrents() {
  case "$PLUCK_CLIENT" in
    transmission)
      transmission_get_completed_torrents
      ;;
    *)
      log_error "Unsupported client: $PLUCK_CLIENT"
      return 1
      ;;
  esac
}

wait_for_client() {
  case "$PLUCK_CLIENT" in
    transmission)
      log_info "Waiting for Transmission at $PLUCK_CLIENT_URL..."
      # shellcheck disable=SC2046
      until curl -s --max-time 3 $(transmission_auth_header) \
        -o /dev/null -w "%{http_code}" "$PLUCK_CLIENT_URL" 2>/dev/null | grep -q "409"; do
        sleep 5
      done
      ;;
    *)
      log_error "Unsupported client: $PLUCK_CLIENT"
      exit 1
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Core logic — scan and pluck
# ---------------------------------------------------------------------------
pluck_torrents() {
  log_debug "Scanning for completed torrents..."

  get_completed_torrents | while IFS='|' read -r label src; do
    [ -z "$label" ] && continue
    [ -z "$src" ] && continue

    destination_dir=$(get_destination_for_label "$label")

    if [ -z "$destination_dir" ]; then
      log_debug "No rule for label '$label', skipping: $(basename "$src")"
      continue
    fi

    name=$(basename "$src")
    dst="${destination_dir}/${name}"

    if [ -e "$dst" ]; then
      log_debug "Already exists, skipping: $name"
      continue
    fi

    if [ ! -e "$src" ]; then
      log_debug "Source not found, skipping: $src"
      continue
    fi

    if place_file "$src" "$dst"; then
      log_info "Plucked ($PLUCK_MODE): $name -> $destination_dir"
    else
      log_error "Failed to pluck: $name -> $destination_dir"
    fi
  done
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  log_info "pluck starting..."
  log_info "Client: $PLUCK_CLIENT | Mode: $PLUCK_MODE | Watch: $PLUCK_WATCH_DIR"

  if [ "$PLUCK_DRY_RUN" = "true" ]; then
    log_info "Dry-run mode enabled — no files will be moved"
  fi

  load_rules
  wait_for_client

  log_info "Client is ready — running initial scan..."
  pluck_torrents

  log_info "Watching $PLUCK_WATCH_DIR for changes..."

  # Periodic rescan in the background
  (
    while true; do
      sleep "$PLUCK_RESCAN_INTERVAL"
      log_debug "Periodic rescan triggered"
      pluck_torrents
    done
  ) &

  # Filesystem watcher in the foreground
  inotifywait -m -r -e create -e moved_to --format '%w%f' "$PLUCK_WATCH_DIR" | \
  while read -r _path; do
    sleep 2
    pluck_torrents
  done
}

main "$@"
