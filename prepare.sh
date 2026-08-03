#!/usr/bin/env bash

set -euo pipefail

MODE="${1:-test}"
APP_NAME="${APP_NAME:-evilginx}"
SESSION="${SESSION:-$APP_NAME}"
SERVICE_NAME="${SERVICE_NAME:-$APP_NAME}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
WORKDIR="${WORKDIR:-$SCRIPT_DIR}"
BIN_PATH="${BIN_PATH:-$WORKDIR/$APP_NAME}"
PHISHLETS_DIR="${PHISHLETS_DIR:-$WORKDIR/phishlets}"
REDIRECTORS_DIR="${REDIRECTORS_DIR:-$WORKDIR/redirectors}"
RUN_SCRIPT="${RUN_SCRIPT:-$WORKDIR/run.sh}"

case "$MODE" in
    test|preview)
        SERVICE_FILE="${SERVICE_FILE:-$WORKDIR/${SERVICE_NAME}.service}"
        INSTALL_SERVICE=0
        ;;
    deploy)
        SERVICE_FILE="${SERVICE_FILE:-/etc/systemd/system/${SERVICE_NAME}.service}"
        INSTALL_SERVICE=1
        ;;
    -h|--help|help)
        cat <<EOF
Usage:
  ./prepare.sh          Generate run.sh and ${SERVICE_NAME}.service in this directory.
  ./prepare.sh test     Same as above.
  ./prepare.sh deploy   Generate run.sh, install systemd service, reload, and enable it.

Optional environment overrides:
  APP_NAME=$APP_NAME
  SESSION=$SESSION
  SERVICE_NAME=$SERVICE_NAME
  WORKDIR=$WORKDIR
  BIN_PATH=$WORKDIR/$APP_NAME
  PHISHLETS_DIR=$WORKDIR/configfiles
  REDIRECTORS_DIR=$WORKDIR/outdir
EOF
        exit 0
        ;;
    *)
        echo "Unknown mode: $MODE" >&2
        echo "Use './prepare.sh test' or './prepare.sh deploy'." >&2
        exit 1
        ;;
esac

if [[ -n "${SUDO_USER:-}" && "$SUDO_USER" != "root" ]]; then
    SERVICE_USER="$SUDO_USER"
else
    SERVICE_USER="$(id -un)"
fi

quote_shell() {
    printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"
}

require_ubuntu() {
    if [[ ! -r /etc/os-release ]]; then
        echo "Cannot identify OS; this prepare script expects Ubuntu Linux." >&2
        exit 1
    fi

    # shellcheck disable=SC1091
    . /etc/os-release
    if [[ "${ID:-}" != "ubuntu" ]]; then
        echo "Unsupported OS '${ID:-unknown}'; this prepare script expects Ubuntu Linux." >&2
        exit 1
    fi
}

ensure_tmux() {
    if command -v tmux >/dev/null 2>&1; then
        return
    fi

    if ! command -v apt-get >/dev/null 2>&1; then
        echo "tmux is missing and apt-get was not found." >&2
        exit 1
    fi

    echo "Installing tmux..."
    if [[ "$(id -u)" -eq 0 ]]; then
        apt-get update
        apt-get install -y tmux
    else
        sudo apt-get update
        sudo apt-get install -y tmux
    fi
}

write_root_file() {
    local target="$1"
    local content="$2"
    local tmp

    tmp="$(mktemp)"
    printf '%s\n' "$content" > "$tmp"

    if [[ "$(id -u)" -eq 0 ]]; then
        install -m 0644 "$tmp" "$target"
    else
        sudo install -m 0644 "$tmp" "$target"
    fi

    rm -f "$tmp"
}

if [[ "$INSTALL_SERVICE" -eq 1 ]]; then
    require_ubuntu
    ensure_tmux
elif ! command -v tmux >/dev/null 2>&1; then
    echo "Warning: tmux is not installed; generated files will reference /usr/bin/tmux." >&2
fi

if [[ ! -d "$WORKDIR" ]]; then
    echo "Working directory does not exist: $WORKDIR" >&2
    exit 1
fi

if [[ "$INSTALL_SERVICE" -eq 1 && ! -x "$BIN_PATH" ]]; then
    echo "Expected executable is missing or not executable: $BIN_PATH" >&2
    echo "Set BIN_PATH=/path/to/binary if the binary is somewhere else." >&2
    exit 1
elif [[ ! -e "$BIN_PATH" ]]; then
    echo "Warning: expected executable does not exist yet: $BIN_PATH" >&2
elif [[ ! -x "$BIN_PATH" ]]; then
    echo "Warning: expected executable is not executable yet: $BIN_PATH" >&2
fi

mkdir -p "$PHISHLETS_DIR" "$REDIRECTORS_DIR"

TMUX_PATH="$(command -v tmux || true)"
TMUX_PATH="${TMUX_PATH:-/usr/bin/tmux}"
BASH_PATH="$(command -v bash)"

cat > "$RUN_SCRIPT" <<EOF
#!/usr/bin/env bash

set -euo pipefail

quote_shell() {
    printf "'%s'" "\$(printf '%s' "\$1" | sed "s/'/'\\\\''/g")"
}

SESSION=$(quote_shell "$SESSION")
WORKDIR=$(quote_shell "$WORKDIR")
TMUX=$(quote_shell "$TMUX_PATH")
BASH=$(quote_shell "$BASH_PATH")
BIN_PATH=$(quote_shell "$BIN_PATH")
PHISHLETS_DIR=$(quote_shell "$PHISHLETS_DIR")
REDIRECTORS_DIR=$(quote_shell "$REDIRECTORS_DIR")

cd "\$WORKDIR"

if "\$TMUX" has-session -t "\$SESSION" 2>/dev/null; then
    exit 0
fi

APP_COMMAND="\$(quote_shell "\$BIN_PATH") -p \$(quote_shell "\$PHISHLETS_DIR") -t \$(quote_shell "\$REDIRECTORS_DIR"); exec bash"
SHELL_COMMAND="\$(quote_shell "\$BASH") -lc \$(quote_shell "\$APP_COMMAND")"

exec "\$TMUX" new-session -d -s "\$SESSION" "\$SHELL_COMMAND"
EOF

chmod 0755 "$RUN_SCRIPT"

UNIT_CONTENT="[Unit]
Description=${APP_NAME} (tmux)
After=network.target

[Service]
Type=oneshot
User=${SERVICE_USER}
WorkingDirectory=${WORKDIR}

ExecStart=${RUN_SCRIPT}
ExecStop=${TMUX_PATH} kill-session -t ${SESSION}

RemainAfterExit=yes

AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target"

if [[ "$INSTALL_SERVICE" -eq 1 ]]; then
    write_root_file "$SERVICE_FILE" "$UNIT_CONTENT"

    if command -v systemctl >/dev/null 2>&1; then
        if [[ "$(id -u)" -eq 0 ]]; then
            systemctl daemon-reload
            systemctl enable "${SERVICE_NAME}.service"
        else
            sudo systemctl daemon-reload
            sudo systemctl enable "${SERVICE_NAME}.service"
        fi
    fi
else
    printf '%s\n' "$UNIT_CONTENT" > "$SERVICE_FILE"
fi

echo "Prepared $SERVICE_NAME.service ($MODE)"
echo "User: $SERVICE_USER"
echo "WorkingDirectory: $WORKDIR"
echo "Run script: $RUN_SCRIPT"
echo "Service file: $SERVICE_FILE"
