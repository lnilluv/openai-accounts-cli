package cmd

import (
	"fmt"
	"strings"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newPoolCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage pooled provider accounts",
	}

	cmd.AddCommand(newPoolActivateCmd(app), newPoolDeactivateCmd(app), newPoolStatusCmd(app))

	return cmd
}

func newPoolActivateCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "activate",
		Short: "Activate the default OpenAI pool",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pool, err := app.poolService.ActivateDefaultOpenAIPool(cmd.Context())
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Activated pool %s (members: %d)\n", pool.ID, len(pool.Members))
			return nil
		},
	}
}

func newPoolDeactivateCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate the default OpenAI pool",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pool, err := app.poolService.DeactivatePool(cmd.Context(), application.DefaultOpenAIPoolID)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deactivated pool %s\n", pool.ID)
			return nil
		},
	}
}

func newPoolStatusCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show default pool status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pool, err := app.poolService.GetPool(cmd.Context(), application.DefaultOpenAIPoolID)
			if err != nil {
				if err == domain.ErrPoolNotFound {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "pool: default-openai")
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "active: false")
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "members: none")
					return nil
				}
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "pool: %s\n", pool.ID)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "active: %t\n", pool.Active)
			if len(pool.Members) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "members: none")
				return nil
			}

			members := make([]string, 0, len(pool.Members))
			for _, member := range pool.Members {
				status, statusErr := app.service.GetStatus(cmd.Context(), member)
				if statusErr == nil && strings.TrimSpace(status.Account.Name) != "" {
					members = append(members, status.Account.Name)
					continue
				}
				members = append(members, string(member))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "members: %s\n", strings.Join(members, ", "))
			return nil
		},
	}
}
