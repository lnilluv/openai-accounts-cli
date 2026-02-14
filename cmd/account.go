package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAccountCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage accounts",
	}

	cmd.AddCommand(
		newAccountListCmd(app),
	)

	return cmd
}

func newAccountListCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			statuses, err := app.service.GetStatusAll(cmd.Context())
			if err != nil {
				return err
			}

			for _, status := range statuses {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", status.Account.ID, status.Account.Name)
			}

			return nil
		},
	}
}

func newNotImplementedCmd(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("%s: %w", cmd.CommandPath(), errNotImplementedYet)
		},
	}
}
