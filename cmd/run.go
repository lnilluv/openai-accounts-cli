package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newRunCmd(app *app) *cobra.Command {
	var poolID string

	cmd := &cobra.Command{
		Use:                "run -- <command> [args...]",
		Short:              "Run a command with pool-selected account env",
		DisableFlagParsing: false,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("run requires a command after '--'")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var picked domain.AccountID

			active, err := app.continuityService.GetActiveAccountID(cmd.Context(), domain.PoolID(poolID))
			if err != nil {
				return err
			}
			if active != "" {
				eligible, err := app.poolService.IsEligibleAccount(cmd.Context(), domain.PoolID(poolID), active)
				if err != nil {
					return err
				}
				if eligible {
					picked = active
				}
			}

			if picked == "" {
				picked, _, err = app.poolService.PickAccount(cmd.Context(), domain.PoolID(poolID))
				if err != nil {
					return err
				}
			}

			if err := app.continuityService.SetActiveAccountID(cmd.Context(), domain.PoolID(poolID), picked); err != nil {
				return err
			}

			workspaceRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve workspace root: %w", err)
			}
			workspaceRoot = filepath.Clean(workspaceRoot)
			windowFingerprint := envOrDefault("OA_WINDOW_FINGERPRINT", "default")
			logicalSessionID := app.continuityService.ResolveLogicalSessionID(workspaceRoot, windowFingerprint)
			providerSessionID, _, err := app.continuityService.GetOrAttachAccountSession(cmd.Context(), domain.PoolID(poolID), logicalSessionID, picked)
			if err != nil {
				return fmt.Errorf("resolve provider session: %w", err)
			}

			child := exec.CommandContext(cmd.Context(), args[0], args[1:]...)
			child.Stdout = cmd.OutOrStdout()
			child.Stderr = cmd.ErrOrStderr()
			child.Stdin = cmd.InOrStdin()
			child.Env = append(os.Environ(),
				"OA_POOL_ID="+poolID,
				"OA_ACTIVE_ACCOUNT="+string(picked),
				"OA_LOGICAL_SESSION_ID="+logicalSessionID,
				"OA_PROVIDER_SESSION_ID="+providerSessionID,
			)

			if err := child.Run(); err != nil {
				return fmt.Errorf("run child command: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&poolID, "pool", string(application.DefaultOpenAIPoolID), "Pool ID")

	return cmd
}
