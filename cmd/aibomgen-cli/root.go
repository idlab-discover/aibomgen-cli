package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd represents the base command.
var RootCmd = &cobra.Command{
	Use:   "aibomgen-cli",
	Short: "BOM Generator for Software Projects using AI {}",
	Long:  longDescription,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		initUIAndBanner(cmd)
		return initConfig()
	},

	// When invoked without a subcommand, show help (with banner) instead of.
	// printing a plain usage output.
	RunE: func(cmd *cobra.Command, args []string) error {
		initUIAndBanner(cmd)
		return cmd.Help()
	},
}

var cfgFile string
var renderedBanner string

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,.
	// will be global for your application.

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.aibomgen-cli.yaml or ./config/defaults.yaml)")

	// Ensure `--help` (and help subcommands) show a green banner consistently.
	defaultHelp := RootCmd.HelpFunc()
	RootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		initUIAndBanner(cmd)
		defaultHelp(cmd, args)
	})

	// Suppress usage output on errors – it is noise for a large CLI; the user.
	// should run the subcommand with --help to see usage when needed.
	RootCmd.SilenceUsage = true

	// Add subcommands.
	RootCmd.AddCommand(generateCmd, scanCmd, enrichCmd, validateCmd, completenessCmd, mergeCmd, vulnScanCmd)
}

func initConfig() error {
	// Enable environment variable support up-front so overrides apply regardless
	// of how (or whether) the config file is located below. Previously this
	// block only ran in the `cfgFile != ""` branch, so users without `--config`
	// silently lost all AIBOMGEN_* env vars (issue #9). Replace dots and dashes
	// with underscores so keys like generate.hf-token map to
	// AIBOMGEN_GENERATE_HF_TOKEN.
	viper.SetEnvPrefix("AIBOMGEN")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	notFound := &viper.ConfigFileNotFoundError{}
	var err error
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		err = viper.ReadInConfig()
	} else {
		viper.SetConfigType("yaml")
		// Without a home directory, skip it: the config file is optional.
		if home, herr := os.UserHomeDir(); herr == nil {
			viper.AddConfigPath(home)
		}
		viper.AddConfigPath("./config")

		// Try .aibomgen-cli first, then defaults.yaml.
		viper.SetConfigName(".aibomgen-cli")
		err = viper.ReadInConfig()
		if errors.As(err, notFound) {
			viper.SetConfigName("defaults")
			err = viper.ReadInConfig()
		}
	}

	switch {
	case err == nil:
		configMsg := ui.Dim.Render("Using config file: ") + ui.Secondary.Render(viper.ConfigFileUsed())
		fmt.Fprintln(os.Stderr, configMsg)
	case !errors.As(err, notFound):
		return fmt.Errorf("reading config file: %w", err)
	}
	// The config file is optional, we shouldn't exit when the config is not found.
	return nil
}

const longDescription = "BOM Generator for Software Projects using AI. Helps PDE manufacturers create accurate Bills of Materials for their AI-based software projects."

func initUIAndBanner(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if renderedBanner == "" {
		renderedBanner = ui.Secondary.Render(ui.BannerASCII) + "\n" + longDescription
	}
	cmd.Root().Long = renderedBanner
}
