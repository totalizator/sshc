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
env labels, and last-used timestamps are discarded; directives, other comments,
indentation, and ordering are left untouched. The original is backed up to
<config>.sshc.bak before writing.

Operates on the writable config only (the first --config path, or
~/.ssh/config by default).`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := configPaths
		if len(paths) == 0 {
			paths = defaultConfigPaths()
		}
		if len(paths) == 0 {
			return fmt.Errorf("no config path resolved")
		}
		path := paths[0]

		if cleanDryRun {
			hosts, err := ssh.Load([]string{path})
			if err != nil {
				return err
			}
			// Mirror StripMeta's predicate exactly so preview == apply: a host
			// carries sshc metadata iff at least one recognised field parsed.
			n := 0
			for _, h := range hosts {
				if len(h.Tags) > 0 || h.Env != "" || h.Fav || !h.LastUsed.IsZero() {
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
		fmt.Printf("stripped sshc metadata from %d host(s) in %s (backup: %s.sshc.bak)\n", n, path, path)
		return nil
	},
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "report what would be stripped without writing")
	rootCmd.AddCommand(cleanCmd)
}
