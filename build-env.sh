# build-env.sh — sourced by build.sh
# Provides: varlock/env loading, env bootstrapping, setup wizard
# Depends on: SCRIPT_DIR, _require_cmd (from build.sh)

# ── Env ────────────────────────────────────────────────────────────

_bootstrap_env_local() {
    local schema="$SCRIPT_DIR/.env.schema"
    local env_local="$SCRIPT_DIR/.env.local"

    if [[ -f "$env_local" ]]; then
        return 0
    fi

    if [[ ! -f "$schema" ]]; then
        echo "✗ .env.schema not found"
        exit 1
    fi

    echo "→ bootstrapping .env.local from .env.schema..."

    while IFS='|' read -r var desc; do
        if [[ -n "$var" ]]; then
            echo "${var}=varlock(prompt)" >> "$env_local"
        fi
    done < <(_get_sensitive_vars)

    echo "✓ .env.local created — varlock will prompt for secrets on first run"
}

_get_sensitive_vars() {
    local schema="$SCRIPT_DIR/.env.schema"
    local comments=() line var is_sensitive desc stripped c

    while IFS= read -r line || [[ -n "$line" ]]; do
        if [[ "$line" =~ ^#[[:space:]]*@ ]]; then
            comments+=("$line")
        elif [[ "$line" =~ ^#[[:space:]] ]]; then
            comments+=("$line")
        elif [[ "$line" =~ ^[A-Z][A-Z_]*= ]]; then
            var="${line%%=*}"
            is_sensitive=0
            desc=""

            for c in ${comments[@]+"${comments[@]}"}; do
                stripped="${c#\#}"
                stripped="${stripped# }"
                if [[ "$stripped" == *"@sensitive"* ]]; then
                    is_sensitive=1
                elif [[ "$stripped" != @* ]]; then
                    if [[ -n "$desc" ]]; then desc+=" "; fi
                    desc+="$stripped"
                fi
            done

            if [[ $is_sensitive -eq 1 ]]; then
                echo "${var}|${desc}"
            fi
            comments=()
        elif [[ -z "$line" ]]; then
            comments=()
        fi
    done < "$schema"
}

_load_env() {
    if ! command -v varlock &>/dev/null; then
        echo "✗ varlock not found. Install with: brew install dmno-dev/tap/varlock"
        exit 1
    fi

    _bootstrap_env_local

    local varlock_output
    varlock_output="$(varlock load --path "$SCRIPT_DIR/" --format shell 2>&1)" || {
        echo "✗ Varlock validation failed:"
        echo "$varlock_output"
        exit 1
    }

    set -a
    eval "$varlock_output"
    set +a

    export UPSTREAM_URL="${UPSTREAM_URL:?UPSTREAM_URL not set}"
    export UPSTREAM_API_KEY="${UPSTREAM_API_KEY:?UPSTREAM_API_KEY not set}"
    export PORT="${PORT:-8777}"
}

# ── Setup ──────────────────────────────────────────────────────────

_set_env_value() {
    local key="$1"
    local value="$2"
    local envfile="$SCRIPT_DIR/.env.local"
    if grep -q "^${key}=" "$envfile" 2>/dev/null; then
        sed -i '' "s|^${key}=.*|${key}=${value}|" "$envfile"
    else
        echo "${key}=${value}" >> "$envfile"
    fi
}

do_upstream_setup() {
    _require_cmd varlock "Install with: brew install dmno-dev/tap/varlock"

    echo ""
    echo "┌─────────────────────────────────────────────┐"
    echo "│ Upstream provider setup                     │"
    echo "│ Configure the AI model provider to proxy.   │"
    echo "└─────────────────────────────────────────────┘"
    echo ""
    echo "  Polaris forwards requests to an upstream OpenAI-compatible API."
    echo "  Any provider with a /v1/chat/completions endpoint works."
    echo ""
    echo "  Examples:"
    echo "    https://api.openai.com"
    echo "    https://api.anthropic.com"
    echo "    https://api.deepseek.com"
    echo ""
    echo "  Enter the upstream base URL:"
    local upstream_url
    read -p "  > " upstream_url
    if [[ -z "$upstream_url" ]]; then
        echo "  ✗ No URL entered. Aborted."
        return 1
    fi
    _set_env_value "UPSTREAM_URL" "$upstream_url"
    echo "  ✓ UPSTREAM_URL saved to .env.local"

    echo ""
    echo "  Enter the upstream API key:"
    local api_key
    read -p "  > " api_key
    if [[ -z "$api_key" ]]; then
        echo "  ✗ No key entered. Aborted."
        return 1
    fi
    _set_env_value "UPSTREAM_API_KEY" "$api_key"
    echo "  ✓ UPSTREAM_API_KEY saved to .env.local"
    echo "  Note: encrypt with varlock in the future (not yet supported)."
    echo ""
    echo "  Run './build.sh start' to start the proxy."
}

do_setup() {
    _require_cmd varlock "Install with: brew install dmno-dev/tap/varlock"

    echo ""
    echo "┌─────────────────────────────────────────────┐"
    echo "│ Polaris — Local Development Setup           │"
    echo "└─────────────────────────────────────────────┘"
    echo ""

    echo "── Checking prerequisites ──"
    local prereq_failed=0
    for cmd in go golangci-lint varlock; do
        if command -v "$cmd" &>/dev/null; then
            echo "  ✓ $cmd"
        else
            echo "  ✗ $cmd missing"
            prereq_failed=1
        fi
    done
    if [[ $prereq_failed -eq 1 ]]; then
        echo ""
        echo "✗ Prerequisites missing. Run './build.sh doctor' for install instructions."
        return 1
    fi
    echo "  ✓ All prerequisites present"
    echo ""

    # Bootstrap .env.local if missing
    _bootstrap_env_local

    # Check UPSTREAM_URL
    local upstream_url
    upstream_url="$(varlock printenv UPSTREAM_URL --path "$SCRIPT_DIR/" 2>/dev/null)" || true
    if [[ -n "$upstream_url" ]]; then
        echo "── Upstream provider ──"
        echo "  ✓ Already configured: $upstream_url"
    else
        echo "── Upstream provider ──"
        echo "  Upstream not configured. Set up now? [Y/n]"
        local answer
        read -p "  > " answer
        if [[ "$answer" != "n" && "$answer" != "N" ]]; then
            do_upstream_setup
        else
            echo "  Skipped. Run './build.sh upstream-setup' later."
        fi
    fi

    echo ""
    local port_val
    port_val="$(varlock printenv PORT --path "$SCRIPT_DIR/" 2>/dev/null)" || true
    echo "── Port ──"
    echo "  Server port: ${port_val:-8777}"

    echo ""
    echo "── Building ──"
    _require_cmd golangci-lint "Install with: brew install golangci-lint"
    golangci-lint run ./...
    go build -o bin/polaris ./cmd/server 2>&1
    echo "  ✓ Polaris built"

    echo ""
    echo "┌─────────────────────────────────────────────┐"
    echo "│ Setup complete.                             │"
    echo "│                                             │"
    echo "│ Start the server:                           │"
    echo "│   ./build.sh start                          │"
    echo "└─────────────────────────────────────────────┘"
}
