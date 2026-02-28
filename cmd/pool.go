package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newPoolCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage pooled provider accounts",
	}

	cmd.AddCommand(
		newPoolActivateCmd(app),
		newPoolDeactivateCmd(app),
		newPoolStatusCmd(app),
		newPoolNextCmd(app),
		newPoolSwitchCmd(app),
	)

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
					members = append(members, sanitizeForTerminal(status.Account.Name))
					continue
				}
				members = append(members, sanitizeForTerminal(string(member)))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "members: %s\n", strings.Join(members, ", "))
			return nil
		},
	}
}

func newPoolNextCmd(app *app) *cobra.Command {
	var poolID string

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Switch to next eligible account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			current, err := app.continuityService.GetActiveAccountID(cmd.Context(), domain.PoolID(poolID))
			if err != nil {
				return err
			}

			next, err := app.poolService.NextAccount(cmd.Context(), domain.PoolID(poolID), current)
			if err != nil {
				return err
			}

			if err := app.continuityService.SetActiveAccountID(cmd.Context(), domain.PoolID(poolID), next); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Switched to account %s\n", next)
			return nil
		},
	}

	cmd.Flags().StringVar(&poolID, "pool", string(application.DefaultOpenAIPoolID), "Pool ID")

	return cmd
}

func newPoolSwitchCmd(app *app) *cobra.Command {
	var poolID string
	var accountSelector string

	cmd := &cobra.Command{
		Use:   "switch",
		Short: "Switch to a specific eligible account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			eligible, err := app.poolService.EligibleAccounts(cmd.Context(), domain.PoolID(poolID))
			if err != nil {
				return err
			}

			target, err := resolveSwitchTarget(cmd, app, eligible, accountSelector)
			if err != nil {
				return err
			}

			if err := app.continuityService.SetActiveAccountID(cmd.Context(), domain.PoolID(poolID), target.ID); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Switched to account %s\n", target.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&poolID, "pool", string(application.DefaultOpenAIPoolID), "Pool ID")
	cmd.Flags().StringVar(&accountSelector, "account", "", "Target account ID or name")

	return cmd
}

func resolveSwitchTarget(cmd *cobra.Command, app *app, eligible []domain.Account, selector string) (domain.Account, error) {
	trimmed := strings.TrimSpace(selector)
	if trimmed != "" {
		for _, account := range eligible {
			if string(account.ID) == trimmed {
				return account, nil
			}
			name := displayAccountName(app, cmd, account)
			if strings.EqualFold(name, trimmed) {
				return account, nil
			}
		}
		return domain.Account{}, fmt.Errorf("account %q is not eligible in pool", selector)
	}

	for i, account := range eligible {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d) %s\n", i+1, displayAccountName(app, cmd, account))
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Select account [1-%d]: ", len(eligible))

	reader := bufio.NewReader(cmd.InOrStdin())
	input, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return domain.Account{}, fmt.Errorf("read account selection: %w", err)
	}

	choice, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return domain.Account{}, fmt.Errorf("invalid selection %q", strings.TrimSpace(input))
	}
	if choice < 1 || choice > len(eligible) {
		return domain.Account{}, fmt.Errorf("selection out of range: %d", choice)
	}

	return eligible[choice-1], nil
}

func displayAccountName(app *app, cmd *cobra.Command, account domain.Account) string {
	status, err := app.service.GetStatus(cmd.Context(), account.ID)
	if err == nil && strings.TrimSpace(status.Account.Name) != "" {
		return sanitizeForTerminal(status.Account.Name)
	}

	if strings.TrimSpace(account.Name) != "" {
		return sanitizeForTerminal(account.Name)
	}

	return sanitizeForTerminal(string(account.ID))
}

func sanitizeForTerminal(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}
