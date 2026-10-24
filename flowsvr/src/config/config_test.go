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
[COMMON]
port = 41555

[MySQL]
url = "127.0.0.1:3306"
user = "root"
db_name = "asyncflow"

[Redis]
url = "127.0.0.1:6379"
`)

	var c TomlConfig

	if err := LoadConfigFromRoot(
		root, &c,
	); err != nil {
		t.Fatal(err)
	}

	if c.Common.Port != 41555 ||
		c.MySQL.Dbname != "asyncflow" {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestMissingMySQLURL(t *testing.T) {
	root := t.TempDir()

	writeConfig(t, root, `
[COMMON]
port = 41555

[MySQL]
user = "root"
db_name = "asyncflow"

[Redis]
url = "127.0.0.1:6379"
`)

	var c TomlConfig

	err := LoadConfigFromRoot(root, &c)

	if err == nil ||
		!strings.Contains(
			err.Error(), "mysql.url",
		) {
		t.Fatalf("error=%v", err)
	}
}
