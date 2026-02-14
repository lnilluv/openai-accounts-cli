package pass

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bnema/openai-accounts-cli/internal/ports"
)

var ErrUnavailable = errors.New("pass command unavailable")

type runFunc func(ctx context.Context, input string, args ...string) (stdout string, stderr string, err error)

type Store struct {
	run runFunc
}

var _ ports.SecretStore = (*Store)(nil)

func NewStore() *Store {
	return &Store{run: runPassCommand}
}

func (s *Store) Put(ctx context.Context, key string, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, stderr, err := s.run(ctx, value+"\n", "insert", "-m", "-f", key)
	if err != nil {
		return formatError("put", key, err, stderr)
	}

	return nil
}

func (s *Store) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	stdout, stderr, err := s.run(ctx, "", "show", key)
	if err != nil {
		return "", formatError("get", key, err, stderr)
	}

	stdout = strings.TrimSuffix(stdout, "\n")
	stdout = strings.TrimSuffix(stdout, "\r")

	return stdout, nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, stderr, err := s.run(ctx, "", "rm", "-f", key)
	if err != nil {
		return formatError("delete", key, err, stderr)
	}

	return nil
}

func runPassCommand(ctx context.Context, input string, args ...string) (string, string, error) {
	path, err := exec.LookPath("pass")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", "", ErrUnavailable
		}
		return "", "", fmt.Errorf("locate pass command: %w", err)
	}

	cmd := exec.CommandContext(ctx, path, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	return stdout.String(), strings.TrimSpace(stderr.String()), err
}

func formatError(op string, key string, err error, stderr string) error {
	if stderr == "" {
		return fmt.Errorf("pass %s %q: %w", op, key, err)
	}

	return fmt.Errorf("pass %s %q: %w: %s", op, key, err, stderr)
}
