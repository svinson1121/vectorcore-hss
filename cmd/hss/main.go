package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
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
	"github.com/svinson1121/vectorcore-hss/internal/fivegc"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/gsup"
	"github.com/svinson1121/vectorcore-hss/internal/metrics"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/pcf"
	"github.com/svinson1121/vectorcore-hss/internal/peertracker"
	pgstore "github.com/svinson1121/vectorcore-hss/internal/repository/postgres"
	"github.com/svinson1121/vectorcore-hss/internal/taccache"
	"github.com/svinson1121/vectorcore-hss/internal/udm"
	"github.com/svinson1121/vectorcore-hss/internal/version"
	"github.com/svinson1121/vectorcore-hss/internal/zapgorm"
)

// peerListAdapter adapts diameter.PeerTracker to the api.PeerLister interface.
type peerListAdapter struct{ pt *diameter.PeerTracker }
type servicePeerListAdapter struct{ pt *peertracker.Tracker }
type multiServicePeerListAdapter struct{ trackers []*peertracker.Tracker }

// authFailureAdapter adapts s6a.Handlers to the api.AuthFailureLister interface.
type authFailureAdapter struct{ srv *diameter.Server }

func (a *authFailureAdapter) RecentAuthFailures() []api.AuthFailure {
	raw := a.srv.S6aHandlers().RecentAuthFailures()
	out := make([]api.AuthFailure, len(raw))
	for i, f := range raw {
		out[i] = api.AuthFailure{
			IMSI:        f.IMSI,
			Timestamp:   f.Timestamp.Format("2006-01-02T15:04:05Z"),
			Reason:      f.Reason,
			PeerAddr:    f.PeerAddr,
			AuthScope:   f.AuthScope,
			VisitedPLMN: f.VisitedPLMN,
			VisitedMCC:  f.VisitedMCC,
			VisitedMNC:  f.VisitedMNC,
		}
	}
	return out
}

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

func (a *servicePeerListAdapter) List() []api.ServicePeer {
	raw := a.pt.List()
	out := make([]api.ServicePeer, len(raw))
	for i, p := range raw {
		out[i] = api.ServicePeer{
			Name:       p.Name,
			RemoteAddr: p.RemoteAddr,
			Transport:  p.Transport,
		}
	}
	return out
}

func (a *multiServicePeerListAdapter) List() []api.ServicePeer {
	var out []api.ServicePeer
	seen := make(map[string]struct{})
	for _, tracker := range a.trackers {
		if tracker == nil {
			continue
		}
		for _, p := range tracker.List() {
			key := p.Name + "\x00" + p.RemoteAddr + "\x00" + p.Transport
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, api.ServicePeer{
				Name:       p.Name,
				RemoteAddr: p.RemoteAddr,
				Transport:  p.Transport,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Transport != out[j].Transport {
			return out[i].Transport < out[j].Transport
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].RemoteAddr < out[j].RemoteAddr
	})
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

	// Inherit PLMN from HSS config into 5G NF configs if not explicitly set.
	// This ensures NRF registration advertises the correct home PLMN so that
	// the AUSF can discover our UDM by PLMN (otherwise the NRF defaults to its
	// own PLMN and returns 504 "No SEPP" when the AUSF queries for our PLMN).
	if cfg.UDM.MCC == "" {
		cfg.UDM.MCC = cfg.HSS.MCC
	}
	if cfg.UDM.MNC == "" {
		cfg.UDM.MNC = cfg.HSS.MNC
	}
	if cfg.PCF.MCC == "" {
		cfg.PCF.MCC = cfg.HSS.MCC
	}
	if cfg.PCF.MNC == "" {
		cfg.PCF.MNC = cfg.HSS.MNC
	}

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

	errCh := make(chan error, 5)
	go func() { errCh <- srv.Start() }()

	var gsupSrv *gsup.Server
	if cfg.GSUP.Enabled {
		gsupSrv = gsup.New(cfg.GSUP, cfg.HSS, store, log)
		if georedMgr != nil {
			gsupSrv.WithGeored(georedMgr)
		}
	}

	var udmSrv *udm.Server
	if cfg.UDM.Enabled {
		udmSrv = udm.New(cfg.UDM, store, log)
	}

	var pcfSrv *pcf.Server
	if cfg.PCF.Enabled {
		pcfSrv = pcf.New(cfg.PCF, store, log)
	}

	var fivegcSrv *fivegc.Server
	if udmSrv != nil && pcfSrv != nil && fivegc.Compatible(cfg.UDM, cfg.PCF) {
		fivegcSrv = fivegc.New(udmSrv, pcfSrv, log)
	}

	if cfg.API.Enabled {
		apiSrv := api.New(db, cfg.API, log).WithCLR(srv).WithCache(store).WithPeers(&peerListAdapter{srv.Peers()}).WithAuthFailures(&authFailureAdapter{srv})
		if gsupSrv != nil {
			apiSrv.WithGSUPPeers(&servicePeerListAdapter{gsupSrv.Peers()})
		}
		var sbiTrackers []*peertracker.Tracker
		if udmSrv != nil {
			sbiTrackers = append(sbiTrackers, udmSrv.Peers())
			sbiTrackers = append(sbiTrackers, udmSrv.ForwardedPeers())
		}
		if pcfSrv != nil {
			sbiTrackers = append(sbiTrackers, pcfSrv.Peers())
			sbiTrackers = append(sbiTrackers, pcfSrv.ForwardedPeers())
		}
		if len(sbiTrackers) == 1 {
			apiSrv.WithSBIPeers(&servicePeerListAdapter{sbiTrackers[0]})
		} else if len(sbiTrackers) > 1 {
			apiSrv.WithSBIPeers(&multiServicePeerListAdapter{trackers: sbiTrackers})
		}
		if tacCache != nil {
			apiSrv.WithTAC(tacCache)
		}
		if georedMgr != nil {
			apiSrv.WithGeored(georedMgr)
		}
		go func() { errCh <- apiSrv.Start() }()
	}

	if cfg.GSUP.Enabled {
		go func() { errCh <- gsupSrv.Start() }()
	}

	if fivegcSrv != nil {
		go func() { errCh <- fivegcSrv.Start() }()
	} else {
		if cfg.UDM.Enabled {
			go func() { errCh <- udmSrv.Start() }()
		}
		if cfg.PCF.Enabled {
			go func() { errCh <- pcfSrv.Start() }()
		}
	}

	readyFields := []zap.Field{
		zap.String("diameter", fmt.Sprintf("%s:%d", cfg.HSS.BindAddress, cfg.HSS.BindPort)),
		zap.String("api", fmt.Sprintf("%s:%d", cfg.API.BindAddress, cfg.API.BindPort)),
	}
	if cfg.GSUP.Enabled {
		readyFields = append(readyFields, zap.String("gsup", fmt.Sprintf("%s:%d", cfg.GSUP.BindAddress, cfg.GSUP.BindPort)))
	}
	if fivegcSrv != nil {
		readyFields = append(readyFields, zap.String("5gc_sbi", fmt.Sprintf("%s:%d", cfg.UDM.BindAddress, cfg.UDM.BindPort)))
		readyFields = append(readyFields, zap.String("udm", "enabled"))
		readyFields = append(readyFields, zap.String("pcf", "enabled"))
	} else {
		if cfg.UDM.Enabled {
			readyFields = append(readyFields, zap.String("udm", fmt.Sprintf("%s:%d", cfg.UDM.BindAddress, cfg.UDM.BindPort)))
		}
		if cfg.PCF.Enabled {
			readyFields = append(readyFields, zap.String("pcf", fmt.Sprintf("%s:%d", cfg.PCF.BindAddress, cfg.PCF.BindPort)))
		}
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
