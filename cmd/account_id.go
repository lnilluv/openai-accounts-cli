package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

func resolveAccountID(ctx context.Context, app *app, raw string) (domain.AccountID, error) {
	requested := strings.TrimSpace(raw)
	if requested == "" || requested == "0" {
		return nextAvailableAccountID(ctx, app)
	}

	if n, err := strconv.Atoi(requested); err == nil && n <= 0 {
		return "", fmt.Errorf("account must be a positive number or empty/0 for auto assignment")
	}

	return domain.AccountID(requested), nil
}

func nextAvailableAccountID(ctx context.Context, app *app) (domain.AccountID, error) {
	statuses, err := app.service.GetStatusAll(ctx)
	if err != nil {
		return "", fmt.Errorf("list accounts for auto assignment: %w", err)
	}

	used := make(map[int]struct{}, len(statuses))
	for _, status := range statuses {
		n, err := strconv.Atoi(string(status.Account.ID))
		if err != nil || n <= 0 {
			continue
		}
		used[n] = struct{}{}
	}

	for i := 1; ; i++ {
		if _, ok := used[i]; !ok {
			return domain.AccountID(strconv.Itoa(i)), nil
		}
	}
}
