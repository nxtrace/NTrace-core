package cmd

import (
	"log"

	"github.com/sjlleo/nexttrace-core/core"
	"github.com/sjlleo/nexttrace-core/plgn"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{Use: "NextTrace"}

var cmdPrint = &cobra.Command{
	Use:   "trace",
	Short: "Traceroute",
	Run: func(cmd *cobra.Command, args []string) {
		debugLevel := viper.GetInt("debug-level")
		enabledPlugins := viper.GetString("plugins")

		plgn.RegisterPlugin("debug", plgn.NewDebugPlugin)
		plugins := plgn.CreatePlugins(enabledPlugins, debugLevel)
		core.Traceroute(plugins)
	},
}

// Execute parse subcommand and run
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}

func init() {
	rootCmd.AddCommand(cmdPrint)
	rootCmd.PersistentFlags().Int("debug-level", 1, "Set debug level (1=info, 2=warn, 3=err)")
	rootCmd.PersistentFlags().String("plugins", "default", "Comma-separated list of enabled plugins")

	viper.SetDefault("debug-level", 1)
	viper.SetDefault("plugins", "default")

	viper.BindPFlag("debug-level", rootCmd.PersistentFlags().Lookup("debug-level"))
	viper.BindPFlag("plugins", rootCmd.PersistentFlags().Lookup("plugins"))

}
