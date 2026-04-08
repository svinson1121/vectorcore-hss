// Command diamtest is a Diameter S6a test client for VectorCore HSS.
//
// Usage:
//   diamtest [global-flags] <command> [command-flags]
//
//   diamtest air   --imsi 001010000000001
//   diamtest ulr   --imsi 001010000000001
//   diamtest pur   --imsi 001010000000001
//   diamtest suite --config testdata/suite.yaml
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/svinson1121/vectorcore-hss/cmd/diamtest/testcases"
)

func main() {
	hss := flag.String("hss", "localhost:3868", "HSS host:port")
	originHost := flag.String("origin-host", "diamtest.test.net", "Diameter Origin-Host")
	originRealm := flag.String("origin-realm", "test.net", "Diameter Origin-Realm")
	mcc := flag.String("mcc", "001", "Home MCC (3 digits)")
	mnc := flag.String("mnc", "01", "Home MNC (2 or 3 digits)")
	verbose := flag.Bool("v", false, "verbose output")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	log := buildLogger(*verbose)

	cfg := &testcases.Config{
		HSSAddr:     *hss,
		OriginHost:  *originHost,
		OriginRealm: *originRealm,
		MCC:         *mcc,
		MNC:         *mnc,
		Log:         log,
		Verbose:     *verbose,
	}

	var err error
	switch args[0] {
	case "air":
		err = runAIR(cfg, args[1:])
	case "ulr":
		err = runULR(cfg, args[1:])
	case "pur":
		err = runPUR(cfg, args[1:])
	case "suite":
		err = runSuite(cfg, args[1:])
	case "load":
		err = runLoad(cfg, args[1:])
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
}

func runAIR(cfg *testcases.Config, args []string) error {
	fs := flag.NewFlagSet("air", flag.ExitOnError)
	imsi := fs.String("imsi", "", "IMSI (required)")
	vectors := fs.Uint("vectors", 1, "Number of vectors to request")
	fs.Parse(args) //nolint
	if *imsi == "" {
		return fmt.Errorf("--imsi is required")
	}
	return testcases.SendAIR(cfg, *imsi, cfg.MCC, cfg.MNC, uint32(*vectors))
}

func runULR(cfg *testcases.Config, args []string) error {
	fs := flag.NewFlagSet("ulr", flag.ExitOnError)
	imsi := fs.String("imsi", "", "IMSI (required)")
	ratType := fs.Uint("rat-type", 1004, "RAT-Type (1004=EUTRAN)")
	fs.Parse(args) //nolint
	if *imsi == "" {
		return fmt.Errorf("--imsi is required")
	}
	return testcases.SendULR(cfg, *imsi, cfg.MCC, cfg.MNC, uint32(*ratType))
}

func runPUR(cfg *testcases.Config, args []string) error {
	fs := flag.NewFlagSet("pur", flag.ExitOnError)
	imsi := fs.String("imsi", "", "IMSI (required)")
	fs.Parse(args) //nolint
	if *imsi == "" {
		return fmt.Errorf("--imsi is required")
	}
	return testcases.SendPUR(cfg, *imsi)
}

func runLoad(cfg *testcases.Config, args []string) error {
	fs := flag.NewFlagSet("load", flag.ExitOnError)
	workers := fs.Int("workers", 10, "Number of concurrent workers")
	duration := fs.Duration("duration", 30*time.Second, "Test duration (ignored if --count is set)")
	count := fs.Int64("count", 0, "Stop after this many requests (0 = use --duration)")
	imsiBase := fs.String("imsi-base", "001010000000001", "Base IMSI (15-digit numeric)")
	imsiCount := fs.Int64("imsi-count", 100, "IMSI pool size (cycles through base..base+count-1)")
	command := fs.String("command", "air", "Diameter command to send: air, ulr, both")
	reqTimeout := fs.Duration("timeout", 5*time.Second, "Per-request timeout")
	fs.Parse(args) //nolint

	switch *command {
	case "air", "ulr", "both":
	default:
		return fmt.Errorf("--command must be air, ulr, or both")
	}

	lc := &testcases.LoadConfig{
		Workers:    *workers,
		Duration:   *duration,
		Count:      *count,
		IMSIBase:   *imsiBase,
		IMSICount:  *imsiCount,
		Command:    *command,
		ReqTimeout: *reqTimeout,
	}
	return testcases.RunLoad(cfg, lc)
}

func runSuite(cfg *testcases.Config, args []string) error {
	fs := flag.NewFlagSet("suite", flag.ExitOnError)
	config := fs.String("config", "cmd/diamtest/testdata/suite.yaml", "path to test suite YAML")
	fs.Parse(args) //nolint
	return testcases.RunSuite(cfg, *config)
}

func usage() {
	fmt.Println(`diamtest — VectorCore HSS Diameter test client

Commands:
  air    Send Authentication-Information-Request
  ulr    Send Update-Location-Request
  pur    Send Purge-UE-Request
  suite  Run a YAML-defined test suite
  load   Run a Diameter load test

Global flags (before the command):
  --hss          HSS address        (default: localhost:3868)
  --mcc          MCC                (default: 001)
  --mnc          MNC                (default: 01)
  --origin-host  Diameter identity  (default: diamtest.test.net)
  --origin-realm Diameter realm     (default: test.net)
  -v             Verbose output

Examples:
  diamtest air --imsi 001010000000001
  diamtest --mcc 001 --mnc 01 air --imsi 001010000000001
  diamtest --mcc 001 --mnc 01 ulr --imsi 001010000000001
  diamtest --mcc 001 --mnc 01 pur --imsi 001010000000001
  diamtest suite --config cmd/diamtest/testdata/suite.yaml
  diamtest load --workers 10 --duration 60s --command air --imsi-base 001010000000001 --imsi-count 1000`)
}

func buildLogger(verbose bool) *zap.Logger {
	level := zapcore.InfoLevel
	if verbose {
		level = zapcore.DebugLevel
	}
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	log, _ := cfg.Build()
	return log
}
