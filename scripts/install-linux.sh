#!/usr/bin/env bash
# Installs dwc-server-advanced on Linux as a systemd service.
#
#   curl -fsSL https://raw.githubusercontent.com/WiiListZ/dwc-server-advanced/main/scripts/install-linux.sh | sudo bash
#
# Environment:
#   DWC_REPOSITORY  GitHub "owner/name" to install from (default WiiListZ/dwc-server-advanced)
#   DWC_VERSION     tag to install (default: latest release)
#   DWC_BINARY      install this local binary instead of downloading one
#   DWC_NO_SERVICE  set to 1 to skip the systemd service
set -euo pipefail

REPOSITORY="${DWC_REPOSITORY:-WiiListZ/dwc-server-advanced}"
INSTALL_DIR=/opt/dwc-server-advanced
DATA_DIR=/var/lib/dwc-server-advanced
SERVICE=dwc-server-advanced
USER_NAME=dwc

say() { printf '\033[1m==>\033[0m %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root (sudo)"
command -v sha256sum >/dev/null || die "sha256sum is required"

case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  armv7l | armv6l) ARCH=arm ;;
  *) die "unsupported architecture $(uname -m)" ;;
esac
ASSET="dwc-server-advanced_linux_${ARCH}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ -n "${DWC_BINARY:-}" ]; then
  say "Using local binary $DWC_BINARY"
  cp "$DWC_BINARY" "$TMP/$ASSET"
else
  command -v curl >/dev/null || die "curl is required"
  if [ -n "${DWC_VERSION:-}" ]; then
    BASE="https://github.com/$REPOSITORY/releases/download/$DWC_VERSION"
  else
    BASE="https://github.com/$REPOSITORY/releases/latest/download"
  fi
  say "Downloading $ASSET from $REPOSITORY"
  curl -fsSL "$BASE/$ASSET" -o "$TMP/$ASSET" || die "no $ASSET in the release; is a release published?"
  curl -fsSL "$BASE/checksums.txt" -o "$TMP/checksums.txt" || die "checksums.txt missing from the release"
  (cd "$TMP" && grep " $ASSET\$" checksums.txt | sha256sum -c -) || die "checksum mismatch"
fi

say "Creating user $USER_NAME and $DATA_DIR"
if ! id "$USER_NAME" >/dev/null 2>&1; then
  useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$USER_NAME" 2>/dev/null \
    || adduser -S -D -H -h "$DATA_DIR" "$USER_NAME"
fi
mkdir -p "$INSTALL_DIR" "$DATA_DIR"
install -m 0755 "$TMP/$ASSET" "$INSTALL_DIR/dwc-server-advanced"
chown -R "$USER_NAME:" "$INSTALL_DIR" "$DATA_DIR"
ln -sf "$INSTALL_DIR/dwc-server-advanced" /usr/local/bin/dwc-server-advanced
say "Installed $("$INSTALL_DIR/dwc-server-advanced" --version 2>/dev/null || echo dwc-server-advanced) to $INSTALL_DIR"

if [ "${DWC_NO_SERVICE:-}" = 1 ] || ! command -v systemctl >/dev/null || [ ! -d /run/systemd/system ]; then
  say "systemd not available: start the server with"
  echo "    sudo -u $USER_NAME dwc-server-advanced --dir $DATA_DIR"
  exit 0
fi

cat > "/etc/systemd/system/$SERVICE.service" <<UNIT
[Unit]
Description=DWC Server Advanced (Nintendo Wi-Fi Connection replacement)
After=network-online.target postgresql.service docker.service
Wants=network-online.target

[Service]
User=$USER_NAME
Group=$USER_NAME
ExecStart=$INSTALL_DIR/dwc-server-advanced --dir $DATA_DIR
Restart=always
RestartSec=2
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now "$SERVICE"
say "Service $SERVICE started"
if [ ! -f "$DATA_DIR/config.yml" ]; then
  echo "    First run: get the setup link with   journalctl -u $SERVICE -n 30 --no-pager"
fi
