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

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "aibomgen-cli",
	Short: "BOM Generator for Software Projects using AI {}",
	Long:  longDescription,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initUIAndBanner(cmd)
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

// SetVersion sets the version for the CLI.
func SetVersion(v string) {
	rootCmd.Version = v
}

// GetRootCmd returns the root command for use with fang.
func GetRootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,.
	// will be global for your application.

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.aibomgen-cli.yaml or ./config/defaults.yaml)")

	// Ensure `--help` (and help subcommands) show a green banner consistently.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		initUIAndBanner(cmd)
		defaultHelp(cmd, args)
	})

	// Suppress usage output on errors – it is noise for a large CLI; the user.
	// should run the subcommand with --help to see usage when needed.
	rootCmd.SilenceUsage = true

	// Add subcommands.
	rootCmd.AddCommand(generateCmd, scanCmd, enrichCmd, validateCmd, completenessCmd, mergeCmd, vulnScanCmd)
}

func initConfig() {
	// Enable environment variable support (e.g. AIBOMGEN_HUGGINGFACE_TOKEN)
	// up-front so overrides apply regardless of how (or whether) the config
	// file is located below. Previously this block only ran in the
	// `cfgFile != ""` branch, so users without `--config` silently lost all
	// AIBOMGEN_* env vars (issue #9). Replace dots with underscores:
	// huggingface.token -> AIBOMGEN_HUGGINGFACE_TOKEN.
	viper.SetEnvPrefix("AIBOMGEN")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.SetConfigType("yaml")
		viper.AddConfigPath(home)
		viper.AddConfigPath("./config")

		// Try .aibomgen-cli first.
		viper.SetConfigName(".aibomgen-cli")
		err = viper.ReadInConfig()

		// If not found, try defaults.yaml.
		notFound := &viper.ConfigFileNotFoundError{}
		if err != nil && errors.As(err, notFound) {
			viper.SetConfigName("defaults")
			err = viper.ReadInConfig()
		}

		if err != nil && !errors.As(err, notFound) {
			cobra.CheckErr(err)
		}

		if err == nil {
			configMsg := ui.Dim.Render("Using config file: ") + ui.Secondary.Render(viper.ConfigFileUsed())
			fmt.Fprintln(os.Stderr, configMsg)
		}

		return
	}

	err := viper.ReadInConfig()

	notFound := &viper.ConfigFileNotFoundError{}
	switch {
	case err != nil && !errors.As(err, notFound):
		cobra.CheckErr(err)
	case err != nil && errors.As(err, notFound):
		// The config file is optional, we shouldn't exit when the config is not found.
		break
	default:
		configMsg := ui.Dim.Render("Using config file: ") + ui.Secondary.Render(viper.ConfigFileUsed())
		fmt.Fprintln(os.Stderr, configMsg)
	}
}

const longDescription = "BOM Generator for Software Projects using AI. Helps PDE manufacturers create accurate Bills of Materials for their AI-based software projects."

func initUIAndBanner(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if renderedBanner == "" {
		renderedBanner = ui.RenderGradientBanner(ui.BannerASCII) + "\n" + longDescription
	}
	cmd.Root().Long = renderedBanner
}
