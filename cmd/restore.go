package cmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/totalizator/sshc/ssh"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [N]",
	Short: "List the config's backups, or put backup N back",
	Long: `Without an argument, list the backups sshc keeps of the writable config,
newest first: <config>.sshc.bak, then .sshc.bak.1 … .sshc.bak.4. Each write
that changes the config (add, edit, delete, clean, restore) makes one;
stamping last-used on connect and toggling a favourite don't.

With N, put backup N (as numbered in the list, 1 = newest) back as the
config. The current config becomes the newest backup first, so
"sshc restore 1" right after undoes the restore.

Operates on the writable config only (the first --config path, or
~/.ssh/config by default).`,
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := writableConfig()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if len(args) == 0 {
			listBackups(out, path)
			return nil
		}
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("N must be a backup number from \"sshc restore\", got %q", args[0])
		}
		return restoreBackup(out, path, n)
	},
}

// writableConfig is the config sshc writes: the first --config path, or
// ~/.ssh/config.
func writableConfig() (string, error) {
	paths := configPaths
	if len(paths) == 0 {
		paths = defaultConfigPaths()
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no config path resolved")
	}
	return paths[0], nil
}

func listBackups(w io.Writer, path string) {
	backups := ssh.Backups(path)
	if len(backups) == 0 {
		fmt.Fprintf(w, "no backups of %s yet\n", path)
		return
	}
	fmt.Fprintf(w, "backups of %s, newest first:\n", path)
	for i, b := range backups {
		fmt.Fprintf(w, "  %d  saved %-9s %-10s %s\n", i+1, ssh.LastUsedLabel(b.ModTime), hostCount(b.Hosts), b.Path)
	}
	fmt.Fprintln(w, "restore one with: sshc restore N")
}

func restoreBackup(w io.Writer, path string, n int) error {
	backups := ssh.Backups(path)
	if n < 1 || n > len(backups) {
		if len(backups) == 0 {
			return fmt.Errorf("no backups of %s to restore", path)
		}
		return fmt.Errorf("no backup %d: there are %d (see \"sshc restore\")", n, len(backups))
	}
	b := backups[n-1]
	if err := ssh.Restore(path, b.Path); err != nil {
		return err
	}
	fmt.Fprintf(w, "restored %s (%s, %s) as %s\n", b.Path, ssh.LastUsedLabel(b.ModTime), hostCount(b.Hosts), path)
	fmt.Fprintln(w, "the config it replaced is now backup 1: \"sshc restore 1\" undoes this")
	return nil
}

func hostCount(n int) string {
	switch n {
	case -1:
		return "unreadable"
	case 1:
		return "1 host"
	}
	return fmt.Sprintf("%d hosts", n)
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
