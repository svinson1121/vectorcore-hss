package testcases

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Suite struct {
	Name  string     `yaml:"name"`
	Tests []TestCase `yaml:"tests"`
}

type TestCase struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Command     string `yaml:"command"`
	IMSI        string `yaml:"imsi"`
	RATType     uint32 `yaml:"rat_type"`
	NumVectors  uint32 `yaml:"num_vectors"`
	Expect      string `yaml:"expect"` // success, user_unknown, auth_unavailable, unknown_eps
}

func RunSuite(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read suite %s: %w", path, err)
	}
	var suite Suite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return fmt.Errorf("parse suite: %w", err)
	}

	cfg.Log.Info("running test suite",
		zap.String("name", suite.Name),
		zap.Int("tests", len(suite.Tests)),
		zap.String("hss", cfg.HSSAddr),
		zap.String("mcc", cfg.MCC),
		zap.String("mnc", cfg.MNC),
	)

	passed, failed := 0, 0
	start := time.Now()

	for i, tc := range suite.Tests {
		cfg.Log.Info(fmt.Sprintf("── %d/%d: %s", i+1, len(suite.Tests), tc.Name))
		if tc.Description != "" {
			cfg.Log.Info("   " + tc.Description)
		}

		err := runTestCase(cfg, &tc)
		wantFailure := tc.Expect != "success" && tc.Expect != ""

		pass := false
		if wantFailure {
			pass = err != nil // we expected an error and got one
		} else {
			pass = err == nil // we expected success and got it
		}

		if pass {
			cfg.Log.Info("  ✓ PASS")
			passed++
		} else {
			if wantFailure {
				cfg.Log.Error("  ✗ FAIL — expected error but got success",
					zap.String("expected", tc.Expect))
			} else {
				cfg.Log.Error("  ✗ FAIL", zap.Error(err))
			}
			failed++
		}
	}

	elapsed := time.Since(start)
	cfg.Log.Info("──────────────────────────")
	cfg.Log.Info("suite complete",
		zap.Int("passed", passed),
		zap.Int("failed", failed),
		zap.Int("total", len(suite.Tests)),
		zap.Duration("elapsed", elapsed),
	)

	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

func runTestCase(cfg *Config, tc *TestCase) error {
	ratType := tc.RATType
	if ratType == 0 {
		ratType = 1004
	}
	numVectors := tc.NumVectors
	if numVectors == 0 {
		numVectors = 1
	}

	switch tc.Command {
	case "air":
		return SendAIR(cfg, tc.IMSI, cfg.MCC, cfg.MNC, numVectors)
	case "ulr":
		return SendULR(cfg, tc.IMSI, cfg.MCC, cfg.MNC, ratType)
	case "pur":
		return SendPUR(cfg, tc.IMSI)
	default:
		return fmt.Errorf("unknown command %q", tc.Command)
	}
}
