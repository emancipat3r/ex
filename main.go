package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vishvananda/netns"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already prints the error; just exit non-zero
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "ex <netns> <command> [args...]",
	Short: "Run a command inside a Linux network namespace",
	Long: `ex is a small helper that runs a command inside a given Linux
network namespace.

Example:
  ex vps ip a
  ex vps curl https://ifconfig.io
  ex myns bash`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		nsName := args[0]
		cmdName := args[1]
		cmdArgs := []string{}
		if len(args) > 2 {
			cmdArgs = args[2:]
		}

		// Save original namespace so we can restore it
		origNS, err := netns.Get()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting current netns: %v\n", err)
			os.Exit(1)
		}
		defer origNS.Close()

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
		// Restore original NS on exit
		defer func() {
			_ = netns.Set(origNS)
		}()

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
