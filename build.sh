#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$SCRIPT_DIR/.watch/polaris.pid"
LOG_FILE="$SCRIPT_DIR/.watch/polaris.log"

TEST_TIMEOUT=120
STARTUP_DELAY=1
RESTART_DELAY=0.3
WATCH_COOLDOWN=1.0

# ── Helpers ──────────────────────────────────────────────────────────

_require_cmd() {
    local cmd="$1" hint="${2:-}"
    if ! command -v "$cmd" &>/dev/null; then
        echo "✗ $cmd not found."
        [[ -n "$hint" ]] && echo "  $hint"
        exit 1
    fi
}

_pid_running() {
    [[ -f "$1" ]] && kill -0 "$(cat "$1")" 2>/dev/null
}

_load_env() {
    if [[ -f "$SCRIPT_DIR/.env" ]]; then
        set -a
        source "$SCRIPT_DIR/.env"
        set +a
    fi
}

# ── Guards ──────────────────────────────────────────────────────────

_guard_not_running() {
    if _pid_running "$PID_FILE"; then
        echo "✗ Server already running (PID $(cat "$PID_FILE"))."
        echo "  Use './build.sh stop' first, then retry."
        exit 1
    fi
}

# ── Build ──────────────────────────────────────────────────────────

_do_build() {
    _require_cmd go "Install with: brew install go"
    _require_cmd golangci-lint "Install with: brew install golangci-lint"
    golangci-lint run ./...
    go build -o bin/polaris ./cmd/server 2>&1
    echo "Polaris built."
}

# ── Local dev commands ─────────────────────────────────────────────

_do_start() {
    _guard_not_running
    _load_env
    _do_build

    mkdir -p "$(dirname "$PID_FILE")"
    nohup "$SCRIPT_DIR/bin/polaris" > "$LOG_FILE" 2>&1 &
    echo "$!" > "$PID_FILE"

    sleep "$STARTUP_DELAY"
    if _pid_running "$PID_FILE"; then
        local port="${POLARIS_PORT:-8777}"
        echo "Polaris started on :$port (PID $(cat "$PID_FILE"))."
    else
        echo "✗ Polaris failed to start. Check $LOG_FILE"
        rm -f "$PID_FILE"
        exit 1
    fi
}

_do_stop() {
    if [[ -f "$PID_FILE" ]]; then
        kill "$(cat "$PID_FILE")" 2>/dev/null || true
        rm -rf "$SCRIPT_DIR/.watch"
        echo "Polaris stopped."
    else
        echo "Polaris is not running."
    fi
}

_do_watch() {
    _require_cmd fswatch "Install with: brew install fswatch"
    _guard_not_running
    _load_env
    _do_build

    mkdir -p "$(dirname "$PID_FILE")"
    nohup "$SCRIPT_DIR/bin/polaris" > "$LOG_FILE" 2>&1 &
    local server_pid=$!
    echo "$server_pid" > "$PID_FILE"

    local port="${POLARIS_PORT:-8777}"
    echo "Polaris started on :$port (PID $server_pid)."
    echo "Watching for Go changes in $SCRIPT_DIR/ ..."
    echo ""

    trap 'kill $server_pid 2>/dev/null; rm -rf "$SCRIPT_DIR/.watch"' EXIT

    while read -r changed; do
        [[ "$changed" =~ \.go$ ]] || continue
        echo "[$(date '+%H:%M:%S')] Change: $(basename "$changed") → rebuilding"
        if golangci-lint run ./... 2>&1 && go build -o bin/polaris ./cmd/server 2>&1; then
            kill "$server_pid" 2>/dev/null || true
            sleep "$RESTART_DELAY"
            nohup "$SCRIPT_DIR/bin/polaris" > "$LOG_FILE" 2>&1 &
            server_pid=$!
            echo "$server_pid" > "$PID_FILE"
            echo "[$(date '+%H:%M:%S')] Restarted (PID $server_pid)."
        else
            echo "[$(date '+%H:%M:%S')] Build failed."
        fi
        echo ""
        while read -r -t 0 _; do :; done
        sleep "$WATCH_COOLDOWN"
    done < <(fswatch --latency 0.5 "$SCRIPT_DIR/" 2>/dev/null)
}

_do_test() {
    _require_cmd go "Install with: brew install go"

    local timeout="${TEST_TIMEOUT}s"
    local -a filtered=()
    local arg
    for arg in "$@"; do
        if [[ "$arg" =~ ^[0-9]+$ ]]; then
            timeout="${arg}s"
        else
            filtered+=("$arg")
        fi
    done

    go test -timeout "$timeout" ./... -v -count=1 ${filtered[@]:+"${filtered[@]}"}
}

_do_lint() {
    _require_cmd golangci-lint "Install with: brew install golangci-lint"
    golangci-lint run ./...
}

_do_clean() {
    rm -rf "$SCRIPT_DIR/bin" "$SCRIPT_DIR/.watch"
    echo "Removed bin/ and .watch/"
}

_do_doctor() {
    local failed=0

    echo "Polaris build environment:"
    for cmd in go golangci-lint fswatch; do
        if command -v "$cmd" &>/dev/null; then
            echo "  ✓ $cmd: $(command -v "$cmd")"
        else
            echo "  ✗ $cmd: missing"
            failed=1
        fi
    done

    return "$failed"
}

usage() {
    cat <<EOF
Usage: ./build.sh <command>
  Local dev:
    build        Compile Go binary
    clean        Remove build artifacts (bin/, .watch/)
    start        Build + start in background
    stop         Stop server
    watch        Start + auto-rebuild on Go changes
    test [N]     Run all Go tests (timeout in seconds, default $TEST_TIMEOUT)
    lint         Run golangci-lint
    doctor       Check build prerequisites
EOF
}

case "${1:-}" in
    build)      _do_build ;;
    clean)      _do_clean ;;
    start)      _do_start ;;
    stop)       _do_stop ;;
    watch)      _do_watch ;;
    test)       shift; _do_test "$@";;
    lint)       _do_lint ;;
    doctor)     _do_doctor ;;
    *)          usage ;;
esac
