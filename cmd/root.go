package cmd

import "github.com/spf13/cobra"

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "oa",
		Short:         "OpenAI Accounts CLI (oa): manage auth and usage limits",
		Long:          "oa (OpenAI Accounts CLI) helps you store account auth references, run OpenAI login flows, fetch usage/limit snapshots, and view account status from the terminal.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	app, err := wireApp()
	if err != nil {
		rootCmd.RunE = func(_ *cobra.Command, _ []string) error {
			return err
		}
		return rootCmd
	}

	rootCmd.AddCommand(
		newVersionCmd(),
		newAccountCmd(app),
		newAuthCmd(app),
		newPoolCmd(app),
		newRunCmd(app),
		newUsageCmd(app),
	)

	return rootCmd
}
