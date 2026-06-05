package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/totalizator/sshc/ssh"
	"github.com/totalizator/sshc/tui"
)

var (
	configPaths  []string
	filterQuery  string
	execTemplate string
	sortName     bool
	noProxy      bool
	showVersion  bool

	uiTheme   string
	uiVariant string
	uiDensity string
	uiDetail  bool
	uiNoPin   bool
	uiVerbose bool
)

var rootCmd = &cobra.Command{
	Use:           "sshc",
	Short:         "Terminal SSH connection manager",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println("sshc", versionString())
			return nil
		}

		paths := configPaths
		if len(paths) == 0 {
			paths = defaultConfigPaths()
		}

		hosts, err := ssh.Load(paths)
		if err != nil {
			return err
		}

		// Settings layer: defaults < config file < explicitly-set CLI flags.
		settings, err := tui.LoadSettings()
		if err != nil {
			fmt.Fprintln(os.Stderr, "sshc: ignoring config:", err)
			settings = tui.DefaultSettings()
		}
		f := cmd.Flags()
		if f.Changed("theme") {
			settings.Theme = uiTheme
		}
		if f.Changed("variant") {
			settings.Variant = tui.ParseVariant(uiVariant)
		}
		if f.Changed("density") {
			settings.Density = tui.ParseDensity(uiDensity)
		}
		if f.Changed("detail") {
			settings.DetailDefault = uiDetail
		}
		if f.Changed("no-pin") {
			settings.PinFavorites = !uiNoPin
		}
		if f.Changed("verbose-rows") {
			settings.VerboseDefault = uiVerbose
		}

		return tui.Run(tui.Options{
			ConfigPaths: paths,
			Hosts:       hosts,
			Filter:      filterQuery,
			Template:    execTemplate,
			SortByName:  sortName,
			ShowProxy:   !noProxy,
			Version:     versionString(),
			Settings:    settings,
		})
	},
}

// defaultConfigPaths returns ~/.ssh/config (writable) plus the system-wide
// config as a read-only fallback.
func defaultConfigPaths() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".ssh", "config"))
	}
	paths = append(paths, "/etc/ssh/ssh_config")
	return paths
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sshc:", err)
		os.Exit(1)
	}
}

func init() {
	// --config is persistent so subcommands (e.g. `clean`) share it.
	rootCmd.PersistentFlags().StringArrayVar(&configPaths, "config", nil, "SSH config path (repeatable; first is writable, rest read-only)")

	f := rootCmd.Flags()
	f.StringVar(&filterQuery, "filter", "", "pre-filter query on startup")
	f.StringVar(&execTemplate, "template", `ssh "{{{name}}}"`, "exec template")
	f.BoolVar(&sortName, "sort-name", false, "sort list alphabetically by alias")
	f.BoolVar(&noProxy, "no-proxy", false, "hide proxy command column")
	f.BoolVarP(&showVersion, "version", "v", false, "print version")

	f.StringVar(&uiTheme, "theme", "amber", "accent theme: amber | teal | green | magenta")
	f.StringVar(&uiVariant, "variant", "framed", "style variant: minimal | framed | rich")
	f.StringVar(&uiDensity, "density", "comfortable", "row density: comfortable | compact")
	f.BoolVar(&uiDetail, "detail", false, "open the detail panel by default")
	f.BoolVar(&uiNoPin, "no-pin", false, "do not float favourites to the top")
	f.BoolVar(&uiVerbose, "verbose-rows", false, "show verbose rows by default")
}
