package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(
	t *testing.T,
	root, body string,
) {
	t.Helper()

	dir := filepath.Join(root, "config")

	if err := os.MkdirAll(
		dir, 0755,
	); err != nil {
		t.Fatal(err)
	}

	err := os.WriteFile(
		filepath.Join(dir, "config.toml"),
		[]byte(body),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigFromRoot(t *testing.T) {
	root := t.TempDir()

	writeConfig(t, root, `
flowsvr_addr = "http://127.0.0.1:41555"
redis_lock_addr = "127.0.0.1:6379"
`)

	var c TomlConfig

	if err := LoadConfigFromRoot(
		root, &c,
	); err != nil {
		t.Fatal(err)
	}

	if c.FlowsvrAddr != "http://127.0.0.1:41555" {
		t.Fatalf(
			"flowsvr_addr = %q",
			c.FlowsvrAddr,
		)
	}
}

func TestMissingFlowsvrAddr(t *testing.T) {
	root := t.TempDir()

	writeConfig(t, root, `
redis_lock_addr = "127.0.0.1:6379"
`)

	var c TomlConfig

	err := LoadConfigFromRoot(root, &c)

	if err == nil ||
		!strings.Contains(
			err.Error(), "flowsvr_addr",
		) {
		t.Fatalf("error=%v", err)
	}
}
