# `ex` — Lightweight Network Namespace Executor

`ex` is a tiny Go-based CLI tool (built with Cobra) that lets you run commands inside a Linux network namespace without typing:

```bash
ip netns exec <ns> <cmd> [args...]
```

With ex, the same thing becomes:

```bash
ex <netns> <command> [args...]
```

It’s fast, minimal, and designed for red team, lab, and container/network-namespace heavy workflows.

## Features

 - Simple positional-argument workflow
 - No sudo required once capabilities are applied
 - Clean, helpful Cobra CLI help text
 - Uses the battle-tested vishvananda/netns library
 - Perfect for namespace-based isolation, tunnels, pivots, mCP setups, etc.

## Usage

```bash
ex <netns> <command> [args...]
```

Examples:

```bash
# Inspect interfaces in the "vps" netns
ex myns ip a

# cURL from inside a namespace
ex myns curl google.com

# Full shell inside a namespace
ex myns bash

# List available namespaces
ex -l
```

Commands run via `ex` get `NETNS=<name>` in their environment, so you can show
the active namespace in your shell prompt, e.g. in `.bashrc`:

```bash
[ -n "$NETNS" ] && PS1="[$NETNS] $PS1"
```

## Installation
1. Clone and build

```bash
git clone https://github.com/<you>/<repo>.git
cd <repo>

go mod tidy
go build -o ex .
```

2. Give it the necessary Linux capabilities

This removes the need for sudo when switching namespaces:

```bash
sudo setcap cap_sys_admin,cap_net_admin,cap_dac_override+ep ./ex
```

 - `cap_sys_admin` — `setns(2)`, `unshare(2)` and bind mounts
 - `cap_net_admin` — network namespace operations
 - `cap_dac_override` — read root-owned `/var/run/netns/<name>` handles and
   `/etc/netns/<name>/*` overrides as a normal user

You can check applied capabilities with:

```bash
getcap ./ex
```

## Dependencies

 - spf13/cobra
   - CLI framework
 - vishvananda/netns
   - Namespace management

Both are automatically pulled in via go mod tidy.

## How It Works

`ex` mirrors `ip netns exec`, minus the typing:

1. Pins itself to a single OS thread (namespace switches are per-thread in Linux).
2. Resolves the command via `$PATH` and opens `/var/run/netns/<name>`.
3. If `/etc/netns/<name>/` exists, enters a private mount namespace and
   bind-mounts every file in it over the matching file in `/etc` (e.g.
   `resolv.conf`, `hosts`) so DNS and host overrides are per-namespace.
   Problems here are warnings, not failures.
4. Switches into the target network namespace with `setns(2)`.
5. Replaces itself with the command via `execve(2)`.

Because it execs rather than forks, `ex` doesn't stay resident: exit codes,
signals (Ctrl-C), job control and terminal ownership all belong to the command
itself, exactly as if you had run it directly.

### Per-namespace DNS

```bash
sudo mkdir -p /etc/netns/myns
echo 'nameserver 1.1.1.1' | sudo tee /etc/netns/myns/resolv.conf
ex myns dig example.com   # uses 1.1.1.1
```
