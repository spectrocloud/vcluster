package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spectrocloud/vcluster/cmd/vclusterctl/cmd/get"
	"github.com/spectrocloud/vcluster/cmd/vclusterctl/flags"
	"github.com/spectrocloud/vcluster/cmd/vclusterctl/log"
	"github.com/spectrocloud/vcluster/pkg/upgrade"
	"github.com/spf13/cobra"
)

// NewRootCmd returns a new root command
func NewRootCmd(log log.Logger) *cobra.Command {
	return &cobra.Command{
		Use:           "vcluster",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "Welcome to vcluster!",
		PersistentPreRun: func(cobraCmd *cobra.Command, args []string) {
			if globalFlags.Silent {
				log.SetLevel(logrus.FatalLevel)
			}
		},
		Long: `vcluster root command`,
	}
}

var globalFlags *flags.GlobalFlags

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	log := log.GetInstance()
	rootCmd := BuildRoot(log)

	// Execute command
	err := rootCmd.Execute()
	if err != nil {
		if globalFlags.Debug {
			log.Fatalf("%+v", err)
		} else {
			log.Fatal(err)
		}
	}
}

// BuildRoot creates a new root command from the
func BuildRoot(log log.Logger) *cobra.Command {
	rootCmd := NewRootCmd(log)
	persistentFlags := rootCmd.PersistentFlags()
	globalFlags = flags.SetGlobalFlags(persistentFlags)

	// Set version for --version flag
	rootCmd.Version = upgrade.GetVersion()

	// add top level commands
	rootCmd.AddCommand(NewConnectCmd(globalFlags))
	rootCmd.AddCommand(NewCreateCmd(globalFlags))
	rootCmd.AddCommand(NewListCmd(globalFlags))
	rootCmd.AddCommand(NewDeleteCmd(globalFlags))
	rootCmd.AddCommand(NewPauseCmd(globalFlags))
	rootCmd.AddCommand(NewResumeCmd(globalFlags))
	rootCmd.AddCommand(NewUpgradeCmd())
	rootCmd.AddCommand(NewCompletionCmd())
	rootCmd.AddCommand(get.NewGetCmd(globalFlags))
	rootCmd.AddCommand(versionCmd)
	return rootCmd
}
