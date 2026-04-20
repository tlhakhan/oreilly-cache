# Deploying oreilly-cache

## Overview

`oreilly-cache` ships as a single self-contained binary. The frontend SPA is embedded at build time — no separate web server or static file host is needed.

## Requirements

- Linux (amd64 or arm64)
- A user account to run the service (created below)
- Network access to `https://learning.oreilly.com`

## Install

1. Download the latest binary from the [releases page](https://github.com/tlhakhan/oreilly-cache/releases).

2. Install it:

```sh
sudo install -m 755 oreilly-cache-linux-amd64 /usr/local/bin/oreilly-cache
```

3. Create a dedicated system user and cache directory:

```sh
sudo useradd --system --no-create-home --shell /usr/sbin/nologin oreilly-cache
sudo mkdir -p /var/lib/oreilly-cache
sudo chown oreilly-cache:oreilly-cache /var/lib/oreilly-cache
```

## Systemd service

Copy the unit file into place and enable it:

```sh
sudo cp oreilly-cache.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now oreilly-cache
```

Check status:

```sh
sudo systemctl status oreilly-cache
sudo journalctl -u oreilly-cache -f
```

## Configuration

The service is configured via command-line flags set in the `ExecStart` line of the unit file. Common overrides:

| Flag | Default | Description |
|------|---------|-------------|
| `-cache-dir` | `/var/lib/oreilly-cache` | Root directory for on-disk cache |
| `-listen` | `:8080` | HTTP listen address |
| `-scrape-interval` | `120h` | How often to re-scrape upstream (default: 5 days) |
| `-workers` | `5` | Max concurrent publisher item scrapes |
| `-page-size` | `100` | Items per upstream page request |
| `-http-timeout` | `30s` | Per-request upstream HTTP timeout |
| `-shutdown-timeout` | `10s` | Graceful shutdown deadline |

To change a flag, edit `/etc/systemd/system/oreilly-cache.service`, then reload and restart:

```sh
sudo systemctl daemon-reload
sudo systemctl restart oreilly-cache
```

## Healthcheck

```sh
curl http://localhost:8080/api/healthz
```

Returns `{"status":"ok","uptime":"...","scrape":{...}}`. The `scrape` field is populated after the first scrape cycle completes.

## Upgrading

```sh
sudo systemctl stop oreilly-cache
sudo install -m 755 oreilly-cache-linux-amd64 /usr/local/bin/oreilly-cache
sudo systemctl start oreilly-cache
```

The on-disk cache is forward-compatible across minor versions — no migration is needed.

## Uninstall

```sh
sudo systemctl disable --now oreilly-cache
sudo rm /etc/systemd/system/oreilly-cache.service
sudo rm /usr/local/bin/oreilly-cache
sudo rm -rf /var/lib/oreilly-cache
sudo userdel oreilly-cache
sudo systemctl daemon-reload
```
