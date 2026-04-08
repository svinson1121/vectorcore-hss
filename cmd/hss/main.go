package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/svinson1121/vectorcore-hss/internal/api"
	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/diameter"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/gsup"
	"github.com/svinson1121/vectorcore-hss/internal/zapgorm"
	"github.com/svinson1121/vectorcore-hss/internal/metrics"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	pgstore "github.com/svinson1121/vectorcore-hss/internal/repository/postgres"
	"github.com/svinson1121/vectorcore-hss/internal/taccache"
	"github.com/svinson1121/vectorcore-hss/internal/udm"
	"github.com/svinson1121/vectorcore-hss/internal/version"
)

// peerListAdapter adapts diameter.PeerTracker to the api.PeerLister interface.
type peerListAdapter struct{ pt *diameter.PeerTracker }

func (a *peerListAdapter) List() []api.ConnectedPeer {
	raw := a.pt.List()
	out := make([]api.ConnectedPeer, len(raw))
	for i, p := range raw {
		out[i] = api.ConnectedPeer{
			OriginHost:  p.OriginHost,
			OriginRealm: p.OriginRealm,
			RemoteAddr:  p.RemoteAddr,
			Transport:   p.Transport,
		}
	}
	return out
}

// printVersion writes the version string to stdout and exits 0.
func printVersion() {
	fmt.Printf("VectorCore HSS v%s (API v%s)\n", version.AppVersion, version.APIVersion)
	os.Exit(0)
}

func main() {
	var cfgPath string
	var showVer, debugMode bool

	flag.StringVar(&cfgPath, "c", "config.yaml", "path to config file")
	flag.BoolVar(&showVer, "version", false, "print version and exit")
	flag.BoolVar(&showVer, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debugMode, "d", false, "force debug log level (overrides config)")
	flag.Parse()

	if showVer {
		printVersion()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "VectorCore HSS: %v\n", err)
		os.Exit(1)
	}

	log := buildLogger(cfg.Logging, debugMode)
	defer log.Sync() //nolint:errcheck

	log.Info("VectorCore HSS starting",
		zap.String("version", version.AppVersion),
		zap.String("api_version", version.APIVersion),
		zap.String("origin_host", cfg.HSS.OriginHost),
		zap.String("origin_realm", cfg.HSS.OriginRealm),
	)

	dsn, err := cfg.Database.DSN()
	if err != nil {
		log.Fatal("DSN error", zap.Error(err))
	}

	gormLevel := gormlogger.Warn
	if cfg.Logging.Level == "debug" || debugMode {
		gormLevel = gormlogger.Info
	}
	gormCfg := &gorm.Config{
		Logger: zapgorm.New(log, gormLevel, 200*time.Millisecond),
	}

	var db *gorm.DB
	switch cfg.Database.Type {
	case "postgresql", "postgres":
		db, err = gorm.Open(postgres.Open(dsn), gormCfg)
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(dsn), gormCfg)
	default:
		log.Fatal("unsupported db_type", zap.String("type", cfg.Database.Type))
	}
	if err != nil {
		log.Fatal("database open failed", zap.Error(err))
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)
	prometheus.MustRegister(metrics.NewDBPoolCollector(sqlDB))

	log.Info("running AutoMigrate")
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		log.Fatal("AutoMigrate failed", zap.Error(err))
	}
	log.Info("AutoMigrate complete")

	store := pgstore.New(db, cfg.EIR.NoMatchResponse)

	// Load the Type Allocation Code (IMEI device database) into memory if enabled.
	// This is used to enrich EIR history records with device make/model and is
	// separate from the RAN "Tracking Area Code" (also TAC) used in cell identity.
	var tacCache *taccache.Cache
	if cfg.EIR.TACDBEnabled {
		tacCache = taccache.New()
		var tacRows []models.TACModel
		if err := db.Find(&tacRows).Error; err != nil {
			log.Warn("tac: failed to load device database from DB — enrichment disabled", zap.Error(err))
			tacCache = nil
		} else {
			entries := make(map[string]taccache.Entry, len(tacRows))
			for _, r := range tacRows {
				entries[r.TAC] = taccache.Entry{Make: r.Make, Model: r.Model}
			}
			tacCache.Load(entries)
			log.Info("tac: device database loaded", zap.Int("entries", tacCache.Len()))
		}
	} else {
		log.Info("tac: device database disabled (tac_db.enabled: false)")
	}

	// GeoRed — start inter-node listener and manager if enabled.
	var georedMgr *geored.Manager
	if cfg.Geored.Enabled {
		if cfg.Geored.NodeID == "" {
			log.Fatal("geored: node_id is required when geored is enabled")
		}
		georedMgr = geored.New(cfg.Geored, store, log)
		if err := geored.StartServer(cfg.Geored, store, log); err != nil {
			log.Fatal("geored: inter-node listener failed", zap.Error(err))
		}
		// Perform an initial resync from all peers so we start with fresh state.
		go georedMgr.TriggerResync(context.Background())
	}

	srv, err := diameter.NewServer(cfg, store, log)
	if err != nil {
		log.Fatal("diameter init failed", zap.Error(err))
	}
	if tacCache != nil {
		srv.WithTAC(tacCache)
	}
	if georedMgr != nil {
		srv.WithGeored(georedMgr)
	}

	errCh := make(chan error, 2)
	go func() { errCh <- srv.Start() }()

	if cfg.API.Enabled {
		apiSrv := api.New(db, cfg.API, log).WithCLR(srv).WithCache(store).WithPeers(&peerListAdapter{srv.Peers()})
		if tacCache != nil {
			apiSrv.WithTAC(tacCache)
		}
		if georedMgr != nil {
			apiSrv.WithGeored(georedMgr)
		}
		go func() { errCh <- apiSrv.Start() }()
	}

	if cfg.GSUP.Enabled {
		gsupSrv := gsup.New(cfg.GSUP, cfg.HSS, store, log)
		if georedMgr != nil {
			gsupSrv.WithGeored(georedMgr)
		}
		go func() { errCh <- gsupSrv.Start() }()
	}

	if cfg.UDM.Enabled {
		udmSrv := udm.New(cfg.UDM, store, log)
		go func() { errCh <- udmSrv.Start() }()
	}

	readyFields := []zap.Field{
		zap.String("diameter", fmt.Sprintf("%s:%d", cfg.HSS.BindAddress, cfg.HSS.BindPort)),
		zap.String("api", fmt.Sprintf("%s:%d", cfg.API.BindAddress, cfg.API.BindPort)),
	}
	if cfg.GSUP.Enabled {
		readyFields = append(readyFields, zap.String("gsup", fmt.Sprintf("%s:%d", cfg.GSUP.BindAddress, cfg.GSUP.BindPort)))
	}
	if cfg.UDM.Enabled {
		readyFields = append(readyFields, zap.String("udm", fmt.Sprintf("%s:%d", cfg.UDM.BindAddress, cfg.UDM.BindPort)))
	}
	log.Info("VectorCore HSS ready", readyFields...)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		log.Info("shutting down", zap.String("signal", sig.String()))
	case err := <-errCh:
		log.Fatal("diameter error", zap.Error(err))
	}
}

func buildLogger(cfg config.LoggingConfig, debugOverride bool) *zap.Logger {
	levelStr := cfg.Level
	if debugOverride {
		levelStr = "debug"
	}

	var l zapcore.Level
	switch levelStr {
	case "debug":
		l = zapcore.DebugLevel
	case "warn", "warning":
		l = zapcore.WarnLevel
	case "error":
		l = zapcore.ErrorLevel
	default:
		l = zapcore.InfoLevel
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// newEnc returns a fresh encoder instance — each core must have its own
	// because zap encoders are stateful buffers and must not be shared.
	newEnc := func() zapcore.Encoder { return zapcore.NewJSONEncoder(encCfg) }

	var cores []zapcore.Core

	if debugOverride {
		cores = append(cores, zapcore.NewCore(newEnc(), zapcore.AddSync(os.Stderr), l))
	}

	if cfg.File != "" {
		f, err := os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// Can't open log file — warn on stderr and continue without it.
			fmt.Fprintf(os.Stderr, "VectorCore HSS: cannot open log file %q: %v\n", cfg.File, err)
		} else {
			cores = append(cores, zapcore.NewCore(newEnc(), zapcore.AddSync(f), l))
		}
	}

	if len(cores) == 0 {
		// All outputs disabled — use a no-op core so the app doesn't panic.
		cores = append(cores, zapcore.NewNopCore())
	}

	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
