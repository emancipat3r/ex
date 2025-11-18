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
sudo setcap cap_sys_admin,cap_net_admin+ep ./ex
```

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

ex performs the following steps internally:

    Opens the target namespace from /var/run/netns/<name>

    Saves the current namespace so it can restore afterward

    Switches into the target network namespace

    Executes the specified command using exec.Command

    Restores the original namespace on exit

Everything is transient — ex does not stay resident or modify namespaces.
