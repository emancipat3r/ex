package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

func init() {
	// unshare(2) and setns(2) only affect the calling OS thread. Pin the main
	// goroutine to its thread so the namespace switches and the final execve
	// all happen on the same thread; otherwise the Go scheduler could migrate
	// us and the command would silently run in the host namespace.
	runtime.LockOSThread()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already prints the error; just exit non-zero
		os.Exit(1)
	}
}

const netnsDir = "/var/run/netns"

func listNetns() {
	entries, err := os.ReadDir(netnsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return // no namespaces yet
		}
		fatalf("cannot read %s: %v", netnsDir, err)
	}
	for _, e := range entries {
		fmt.Println(e.Name())
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ex: "+format+"\n", args...)
	os.Exit(1)
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ex: warning: "+format+"\n", args...)
}

// validNSName rejects names that could escape /var/run/netns or /etc/netns.
func validNSName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsRune(name, '/')
}

// mountNetnsEtc mirrors `ip netns exec`: every regular file in
// /etc/netns/<ns>/ is bind-mounted over the same-named file in /etc, inside a
// private mount namespace so nothing leaks to the rest of the system.
// Failures here are non-fatal: the command should still run in the netns.
func mountNetnsEtc(nsName string) {
	etcDir := filepath.Join("/etc/netns", nsName)
	entries, err := os.ReadDir(etcDir)
	if err != nil {
		return // no per-netns config; nothing to do
	}

	var files []os.DirEntry
	for _, e := range entries {
		if e.Type().IsRegular() {
			files = append(files, e)
		}
	}
	if len(files) == 0 {
		return
	}

	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		warnf("cannot unshare mount namespace, skipping %s: %v", etcDir, err)
		return
	}
	if err := unix.Mount("", "/", "", unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
		warnf("cannot make mounts private, skipping %s: %v", etcDir, err)
		return
	}

	for _, e := range files {
		src := filepath.Join(etcDir, e.Name())
		dst := filepath.Join("/etc", e.Name())

		// Resolve symlinks (e.g. systemd-resolved's /etc/resolv.conf) so the
		// bind lands on the real file; create the target if it doesn't exist.
		if resolved, err := filepath.EvalSymlinks(dst); err == nil {
			dst = resolved
		} else if f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.Close()
		} else {
			warnf("cannot create %s: %v", dst, err)
			continue
		}

		if err := unix.Mount(src, dst, "", unix.MS_BIND, ""); err != nil {
			warnf("cannot bind-mount %s over %s: %v", src, dst, err)
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "ex <netns> <command> [args...] | ex -l",
	Short: "Run a command inside a Linux network namespace",
	Long: `ex is a small helper that runs a command inside a given Linux
network namespace. It behaves like "ip netns exec" but is short to type and
needs no sudo once capabilities are set on the binary.

The child gets NETNS=<netns> in its environment, handy for shell prompts.
"ex -l" (or "ex ls") lists available network namespaces.

Example:
  ex vps ip a
  ex vps curl https://ifconfig.io
  ex myns bash
  ex -l`,
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Handle help/list manually since DisableFlagParsing is set
		if len(args) == 1 {
			switch args[0] {
			case "-l", "--list", "ls":
				listNetns()
				return
			}
		}
		if len(args) < 2 {
			cmd.Help()
			if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				return
			}
			os.Exit(1)
		}

		nsName := args[0]
		if !validNSName(nsName) {
			fatalf("invalid netns name %q", nsName)
		}

		// Resolve the command before switching namespaces
		cmdPath, err := exec.LookPath(args[1])
		if err != nil {
			fatalf("%v", err)
		}

		// Open target namespace
		targetNS, err := netns.GetFromName(nsName)
		if err != nil {
			fatalf("cannot open netns %q: %v", nsName, err)
		}

		// Per-netns /etc overrides (resolv.conf, hosts, ...)
		mountNetnsEtc(nsName)

		// Switch to target namespace
		if err := netns.Set(targetNS); err != nil {
			fatalf("cannot switch to netns %q: %v", nsName, err)
		}
		targetNS.Close()

		// Replace ourselves with the command: exit codes, signals, job
		// control and terminal ownership are then all the child's.
		os.Setenv("NETNS", nsName)
		if err := syscall.Exec(cmdPath, args[1:], os.Environ()); err != nil {
			fatalf("cannot exec %s: %v", cmdPath, err)
		}
	},
}
