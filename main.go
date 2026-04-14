package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already prints the error; just exit non-zero
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:                "ex <netns> <command> [args...]",
	Short:              "Run a command inside a Linux network namespace",
	Long: `ex is a small helper that runs a command inside a given Linux
network namespace.

Example:
  ex vps ip a
  ex vps curl https://ifconfig.io
  ex myns bash`,
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Handle help manually since DisableFlagParsing is set
		if len(args) < 2 {
			cmd.Help()
			if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				return
			}
			os.Exit(1)
		}

		nsName := args[0]
		cmdName := args[1]
		cmdArgs := []string{}
		if len(args) > 2 {
			cmdArgs = args[2:]
		}

		// Bind-mount per-netns resolv.conf if it exists
		netnsResolvConf := filepath.Join("/etc/netns", nsName, "resolv.conf")
		if _, err := os.Stat(netnsResolvConf); err == nil {
			if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
				fmt.Fprintf(os.Stderr, "error unsharing mount namespace: %v\n", err)
				os.Exit(1)
			}
			if err := unix.Mount("", "/", "", unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
				fmt.Fprintf(os.Stderr, "error making mounts private: %v\n", err)
				os.Exit(1)
			}
			if err := unix.Mount(netnsResolvConf, "/etc/resolv.conf", "", unix.MS_BIND, ""); err != nil {
				fmt.Fprintf(os.Stderr, "error bind-mounting %s: %v\n", netnsResolvConf, err)
				os.Exit(1)
			}
		}

		// Open target namespace
		targetNS, err := netns.GetFromName(nsName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening netns %q: %v\n", nsName, err)
			os.Exit(1)
		}
		defer targetNS.Close()

		// Switch to target namespace
		if err := netns.Set(targetNS); err != nil {
			fmt.Fprintf(os.Stderr, "error switching to netns %q: %v\n", nsName, err)
			os.Exit(1)
		}

		// Run the requested command
		child := exec.Command(cmdName, cmdArgs...)
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr

		if err := child.Run(); err != nil {
			// If the child exited with a status code, propagate it
			if exitErr, ok := err.(*exec.ExitError); ok {
				if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					os.Exit(ws.ExitStatus())
				}
			}
			fmt.Fprintf(os.Stderr, "error running command: %v\n", err)
			os.Exit(1)
		}
	},
}
