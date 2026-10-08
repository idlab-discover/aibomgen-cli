package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitConfigMalformedReturnsError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(p, []byte("generate: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := cfgFile
	cfgFile = p
	t.Cleanup(func() { cfgFile = old; viper.Reset() })

	if err := initConfig(); err == nil {
		t.Fatal("want error for malformed config, got nil")
	}
}
