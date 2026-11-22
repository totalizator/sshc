package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/totalizator/sshc/ssh"
)

var cleanDryRun bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove sshc metadata comments, returning the config to plain-ssh form",
	Long: `Strip the "#sshc env=… tags=… fav=1 used=…" metadata comments that sshc
writes on Host lines, returning the config to plain-ssh form. Favourites, tags,
env labels, last-used timestamps, startups and after-login steps are discarded;
directives, other comments, indentation, and ordering are left untouched. The
original is backed up to <config>.sshc.bak before writing ("sshc restore"
puts it back).

Operates on the writable config only (the first --config path, or
~/.ssh/config by default).`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := writableConfig()
		if err != nil {
			return err
		}

		if cleanDryRun {
			hosts, err := ssh.Load([]string{path})
			if err != nil {
				return err
			}
			// HasMeta is StripMeta's own predicate, so preview == apply
			// (startup and after-login steps count too).
			n := 0
			for _, h := range hosts {
				if h.HasMeta() {
					n++
				}
			}
			fmt.Printf("would strip sshc metadata from %d host(s) in %s\n", n, path)
			return nil
		}

		n, err := ssh.StripMeta(path)
		if err != nil {
			return err
		}
		if n == 0 {
			fmt.Printf("no sshc metadata to strip in %s\n", path)
			return nil
		}
		fmt.Printf("stripped sshc metadata from %d host(s) in %s (undo: sshc restore 1)\n", n, path)
		return nil
	},
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "report what would be stripped without writing")
	rootCmd.AddCommand(cleanCmd)
}
