package configloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	root := t.TempDir()

	err := os.WriteFile(
		filepath.Join(root, "main.go"),
		[]byte("package main"),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(root, "config", "nested")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot(child, "main.go")
	if err != nil || got != root {
		t.Fatalf(
			"root=%q err=%v, want %q",
			got, err, root,
		)
	}
}

func TestFindRootMissing(t *testing.T) {
	_, err := FindRoot(
		t.TempDir(),
		"missing-marker-asy",
	)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	path := ConfigPath(root)

	if err := os.MkdirAll(
		filepath.Dir(path), 0755,
	); err != nil {
		t.Fatal(err)
	}

	err := os.WriteFile(
		path,
		[]byte("name = \"hello\"\n"),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	var conf struct {
		Name string `toml:"name"`
	}

	if err := Load(root, &conf); err != nil {
		t.Fatal(err)
	}

	if conf.Name != "hello" {
		t.Fatalf("name = %q", conf.Name)
	}
}
