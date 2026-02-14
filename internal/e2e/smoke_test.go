package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmokeFlow(t *testing.T) {
	home := t.TempDir()
	binaryPath := buildBinary(t)
	require.NoError(t, writeAccountsFixture(home))

	_, stderr, err := runOA(t, binaryPath, home,
		"auth", "set",
		"--account", "acc-1",
		"--method", "api_key",
		"--secret-key", "openai://acc-1/api_key",
		"--secret-value", "sk-test-123",
	)
	require.NoError(t, err, "stderr: %s", stderr)

	stdout, stderr, err := runOA(t, binaryPath, home, "status", "--account", "acc-1")
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Contains(t, stdout, "Primary (acc-1)")
}

func buildBinary(t *testing.T) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "oa-e2e")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/oa")
	cmd.Dir = repoRoot(t)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "build oa binary: %s", string(output))
	return binaryPath
}

func runOA(t *testing.T, binaryPath, home string, args ...string) (string, string, error) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "HOME="+home)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func writeAccountsFixture(home string) error {
	configDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	accounts := `version = 1

[[accounts]]
id = "acc-1"
name = "Primary"

[accounts.metadata]
provider = "openai"
model = "gpt-5"

[accounts.auth]
method = ""
secret_ref = ""
`

	return os.WriteFile(filepath.Join(configDir, "accounts.toml"), []byte(accounts), 0o644)
}
