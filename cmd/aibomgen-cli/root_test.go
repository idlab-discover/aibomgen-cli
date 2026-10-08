package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/idlab-discover/aibomgen-cli/internal/apperr"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/bomio"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/generator"
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

func TestResolveVerbosity(t *testing.T) {
	tests := []struct {
		name    string
		set     map[string]any
		want    int
		wantErr bool
	}{
		{"default", nil, 0, false},
		{"quiet", map[string]any{"quiet": true}, -1, false},
		{"verbose", map[string]any{"verbose": 1}, 1, false},
		{"debug", map[string]any{"verbose": 2}, 2, false},
		{"capped", map[string]any{"verbose": 5}, 2, false},
		{"both", map[string]any{"quiet": true, "verbose": 1}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			for k, v := range tt.set {
				viper.Set(k, v)
			}
			got, err := resolveVerbosity()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSlogLevel(t *testing.T) {
	for v, want := range map[int]slog.Level{-1: slog.LevelError, 0: slog.LevelWarn, 1: slog.LevelInfo, 2: slog.LevelDebug} {
		if got := slogLevel(v); got != want {
			t.Errorf("slogLevel(%d) = %v, want %v", v, got, want)
		}
	}
}

func TestInputFrom(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		flag    string
		want    string
		wantErr bool
	}{
		{"argument", []string{"a.json"}, "", "a.json", false},
		{"flag", nil, "b.json", "b.json", false},
		{"neither", nil, "", "", false},
		{"both", []string{"a.json"}, "b.json", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			c := &cobra.Command{}
			c.Flags().String("input", "", "")
			bindFlags(c, "x")
			if tt.flag != "" {
				_ = c.Flags().Set("input", tt.flag)
			}
			got, err := inputFrom(c, tt.args, "x")
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("got %q, %v; want %q, wantErr %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

// TestValidateJSON runs `validate --json <file>` end to end through RootCmd.
func TestValidateJSON(t *testing.T) {
	dir := t.TempDir()
	boms, err := generator.BuildDummyBOM()
	if err != nil {
		t.Fatal(err)
	}
	bomPath := filepath.Join(dir, "bom.json")
	if err := bomio.WriteBOM(boms[0].BOM, bomPath, ""); err != nil {
		t.Fatal(err)
	}
	emptyCfg := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(emptyCfg, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	// Earlier tests reset viper, which drops the flag bindings made in init().
	viper.Reset()
	t.Cleanup(viper.Reset)
	bindFlags(validateCmd, "validate")
	for _, name := range []string{"quiet", "verbose", "no-input"} {
		_ = viper.BindPFlag(name, RootCmd.PersistentFlags().Lookup(name))
	}

	var out bytes.Buffer
	RootCmd.SetOut(&out)
	RootCmd.SetArgs([]string{"--config", emptyCfg, "validate", "--json", bomPath})
	t.Cleanup(func() { RootCmd.SetOut(nil); RootCmd.SetArgs(nil) })
	if err := RootCmd.Execute(); err != nil && !errors.Is(err, apperr.ErrValidation) {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if _, ok := got["valid"]; !ok {
		t.Fatalf("missing \"valid\" key: %s", out.String())
	}
}
