package diameter

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"
	"github.com/fiorix/go-diameter/v4/diam/sm/smpeer"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/basedict"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/cx"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/gx"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/rx"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/s13"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/s6a"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/s6c"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/sh"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/slh"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/swx"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/zh"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/metrics"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"github.com/svinson1121/vectorcore-hss/internal/taccache"
)

const (
	vendor3GPP = uint32(10415)
	appIDS6a   = uint32(16777251)
)

type Server struct {
	cfg   *config.Config
	store repository.Repository
	log   *zap.Logger
	sm    *sm.StateMachine
	s6a   *s6a.Handlers
	s13   *s13.Handlers
	cxH   *cx.Handlers
	gxH   *gx.Handlers
	ct    *ConnTracker
	pt    *PeerTracker
}

// Peers returns the peer tracker so callers (e.g. the API layer) can list
// directly connected Diameter peers.
func (s *Server) Peers() *PeerTracker { return s.pt }

// S6aHandlers returns the S6a handler set, e.g. so the API layer can read
// the recent auth failure ring buffer.
func (s *Server) S6aHandlers() *s6a.Handlers { return s.s6a }

// WithTAC attaches a TAC cache to the S13 handler so device make/model is
// written into EIR history at check time.
func (s *Server) WithTAC(c *taccache.Cache) *Server {
	s.s13.WithTAC(c)
	return s
}

// WithGeored wires a GeoRed publisher into the Diameter handlers that emit
// replication events (S6a AIR/ULR, Cx SAR, Gx CCR).
func (s *Server) WithGeored(pub geored.TypedPublisher) *Server {
	s.s6a.WithGeored(pub)
	s.cxH.WithGeored(pub)
	s.gxH.WithGeored(pub)
	return s
}

func NewServer(cfg *config.Config, store repository.Repository, log *zap.Logger) (*Server, error) {
	if err := basedict.Load(); err != nil {
		return nil, err
	}
	if err := s13.LoadDict(); err != nil {
		return nil, err
	}
	if err := cx.LoadDict(); err != nil {
		return nil, err
	}
	if err := sh.LoadDict(); err != nil {
		return nil, err
	}
	if err := swx.LoadDict(); err != nil {
		return nil, err
	}
	if err := gx.LoadDict(); err != nil {
		return nil, err
	}
	if err := rx.LoadDict(); err != nil {
		return nil, err
	}
	if err := slh.LoadDict(); err != nil {
		return nil, err
	}
	if err := zh.LoadDict(); err != nil {
		return nil, err
	}
	if err := s6c.LoadDict(); err != nil {
		return nil, err
	}
	if err := s6c.LoadMSISDNSupplement(); err != nil {
		return nil, err
	}

	settings := &sm.Settings{
		OriginHost:       datatype.DiameterIdentity(cfg.HSS.OriginHost),
		OriginRealm:      datatype.DiameterIdentity(cfg.HSS.OriginRealm),
		VendorID:         datatype.Unsigned32(vendor3GPP),
		ProductName:      datatype.UTF8String(cfg.HSS.ProductName),
		OriginStateID:    datatype.Unsigned32(uint32(time.Now().Unix())),
		FirmwareRevision: 1,
	}
	machine := sm.New(settings)

	// Configure outbound client capabilities (used for DWR)
	_ = &sm.Client{
		Dict:               dict.Default,
		Handler:            machine,
		MaxRetransmits:     3,
		RetransmitInterval: time.Second,
		EnableWatchdog:     true,
		WatchdogInterval:   time.Duration(cfg.HSS.DWRInterval) * time.Second,
		SupportedVendorID: []*diam.AVP{
			diam.NewAVP(avp.SupportedVendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
		},
		VendorSpecificApplicationID: []*diam.AVP{
			diam.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
				AVP: []*diam.AVP{
					diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(appIDS6a)),
					diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
				},
			}),
		},
	}

	// Connection tracker: maps peer OriginHost → active diam.Conn
	ct := newConnTracker()

	s6aH := s6a.NewHandlers(cfg, store, log, ct)
	s13H := s13.NewHandlers(cfg, store, log)
	cxH := cx.NewHandlers(cfg, store, log, ct)
	gxH := gx.NewHandlers(cfg, store, log, ct)
	s6cH := s6c.NewHandlers(cfg, store, log, ct)
	s6aH.WithOnRegister(s6cH.SendALSCForIMSI)
	pt := newPeerTracker()
	s := &Server{cfg: cfg, store: store, log: log, sm: machine, s6a: s6aH, s13: s13H, cxH: cxH, gxH: gxH, ct: ct, pt: pt}

	h := s6aH
	shH := sh.NewHandlers(cfg, store, log, ct)
	swxH := swx.NewHandlers(cfg, store, log, ct)
	rxH := rx.NewHandlers(cfg, store, log, ct)
	slhH := slh.NewHandlers(cfg, store, log)
	zhH := zh.NewHandlers(cfg, store, log)
	wrap := func(name string, fn func(diam.Conn, *diam.Message) (*diam.Message, error)) diam.HandlerFunc {
		return func(conn diam.Conn, msg *diam.Message) {
			// Track the peer by its OriginHost so CLR can find it later.
			var hdr struct {
				OriginHost datatype.DiameterIdentity `avp:"Origin-Host"`
			}
			if msg.Unmarshal(&hdr) == nil && hdr.OriginHost != "" {
				ct.Set(string(hdr.OriginHost), conn)
			}

			start := time.Now()
			log.Info("diameter: recv", zap.String("cmd", name), zap.String("transport", conn.RemoteAddr().Network()), zap.String("peer", conn.RemoteAddr().String()))
			ans, err := fn(conn, msg)

			result := "success"
			if err != nil {
				result = "error"
				log.Error("diameter: handler error", zap.String("cmd", name), zap.Error(err))
			}
			metrics.DiameterRequestsTotal.WithLabelValues(name, result).Inc()
			metrics.DiameterRequestDuration.WithLabelValues(name).Observe(time.Since(start).Seconds())

			if ans == nil {
				return
			}
			if _, err := ans.WriteTo(conn); err != nil {
				log.Error("diameter: write failed", zap.String("cmd", name), zap.Error(err))
			}
		}
	}

	machine.HandleFunc(diam.AIR, wrap("AIR", h.AIR))
	machine.HandleFunc(diam.ULR, wrap("ULR", h.ULR))
	machine.HandleFunc(diam.PUR, wrap("PUR", h.PUR))
	machine.HandleFunc("ECR", wrap("ECR", s13H.ECR))
	machine.HandleFunc("UAR", wrap("UAR", cxH.UAR))
	machine.HandleFunc("SAR", wrap("SAR", func(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
		if msg.Header.ApplicationID == swx.AppIDSWx {
			return swxH.SAR(conn, msg)
		}
		return cxH.SAR(conn, msg)
	}))
	machine.HandleFunc("LIR", wrap("LIR", cxH.LIR))
	machine.HandleFunc("MAR", wrap("MAR", func(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
		if msg.Header.ApplicationID == swx.AppIDSWx {
			return swxH.MAR(conn, msg)
		}
		if msg.Header.ApplicationID == zh.AppIDZh {
			return zhH.MAR(conn, msg)
		}
		return cxH.MAR(conn, msg)
	}))
	machine.HandleFunc("UDR", wrap("UDR", shH.UDR))
	machine.HandleFunc("PNA", func(conn diam.Conn, msg *diam.Message) {
		shH.PNA(conn, msg)
	})
	machine.HandleFunc("CCR", wrap("CCR", gxH.CCR))
	machine.HandleFunc("AAR", wrap("AAR", rxH.AAR))
	machine.HandleFunc("STR", wrap("STR", rxH.STR))
	machine.HandleFunc("LRR", wrap("LRR", slhH.LRR))
	machine.HandleFunc("NOR", wrap("NOR", h.NOR))
	machine.HandleFunc("SIR", wrap("SRI-SM", s6cH.SRISR))
	machine.HandleFunc("RDR", wrap("RSDS", s6cH.RDSMR))

	// ASA — Alert-Service-Centre-Answer. The HSS originates ALSC;
	// this handler receives the SMS-SC's reply and deletes MWD on success.
	machine.HandleFunc("ASA", func(conn diam.Conn, msg *diam.Message) {
		s6cH.ASA(conn, msg)
	})

	// CLA — Cancel-Location-Answer. The HSS originates CLR;
	// this handler receives the MME's CLA reply and logs the result.
	machine.HandleFunc("CLA", func(conn diam.Conn, msg *diam.Message) {
		var cla struct {
			OriginHost datatype.DiameterIdentity `avp:"Origin-Host"`
			ResultCode datatype.Unsigned32       `avp:"Result-Code"`
		}
		_ = msg.Unmarshal(&cla)
		log.Info("diameter: CLA received",
			zap.String("peer", conn.RemoteAddr().String()),
			zap.String("origin_host", string(cla.OriginHost)),
			zap.Uint32("result_code", uint32(cla.ResultCode)),
		)
	})

	// DPR — Disconnect-Peer-Request. The state machine does not handle this
	// automatically; we must send a DPA or the peer will not disconnect cleanly.
	machine.HandleFunc("DPR", func(conn diam.Conn, msg *diam.Message) {
		var req struct {
			OriginHost  datatype.DiameterIdentity `avp:"Origin-Host"`
			OriginRealm datatype.DiameterIdentity `avp:"Origin-Realm"`
		}
		_ = msg.Unmarshal(&req)
		log.Info("diameter: peer disconnecting (DPR)",
			zap.String("transport", conn.RemoteAddr().Network()),
			zap.String("peer", conn.RemoteAddr().String()),
			zap.String("origin_host", string(req.OriginHost)),
		)
		ans := msg.Answer(diam.Success)
		ans.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(cfg.HSS.OriginHost))
		ans.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(cfg.HSS.OriginRealm))
		if _, err := ans.WriteTo(conn); err != nil {
			log.Error("diameter: DPA write failed", zap.Error(err))
		}
	})

	// RAA — Re-Auth-Answer (cmd 258, answer flag clear) sent back by the PGW
	// in response to our Gx RAR for dedicated bearer installation.
	// We fire-and-forget the RAR so just log the result here.
	machine.HandleFunc("RAA", func(conn diam.Conn, msg *diam.Message) {
		log.Debug("diameter: RAA received", zap.String("peer", conn.RemoteAddr().String()))
	})

	machine.HandleFunc("ALL", func(conn diam.Conn, msg *diam.Message) {
		log.Warn("diameter: unhandled",
			zap.Uint32("cmd", uint32(msg.Header.CommandCode)),
			zap.String("peer", conn.RemoteAddr().String()),
		)
	})

	// Log peer connections as they complete the CER/CEA handshake.
	// smpeer.Metadata carries the peer's Origin-Host, Origin-Realm, and
	// advertised application IDs extracted from the CER.
	go func() {
		for conn := range machine.HandshakeNotify() {
			remoteAddr := conn.RemoteAddr().String()
			transport := conn.RemoteAddr().Network()
			fields := []zap.Field{
				zap.String("transport", transport),
				zap.String("peer", remoteAddr),
			}
			peer := ConnectedPeer{
				RemoteAddr: remoteAddr,
				Transport:  transport,
			}
			if meta, ok := smpeer.FromContext(conn.Context()); ok {
				fields = append(fields,
					zap.String("origin_host", string(meta.OriginHost)),
					zap.String("origin_realm", string(meta.OriginRealm)),
					zap.Uint32s("apps", meta.Applications),
				)
				peer.OriginHost = string(meta.OriginHost)
				peer.OriginRealm = string(meta.OriginRealm)
			}
			pt.add(peer)
			log.Info("diameter: peer connected", fields...)

			// Remove from tracker and log when the peer disconnects.
			if cn, ok := conn.(diam.CloseNotifier); ok {
				go func(addr string, fields []zap.Field) {
					<-cn.CloseNotify()
					pt.remove(addr)
					log.Info("diameter: peer disconnected", fields...)
				}(remoteAddr, fields)
			}
		}
	}()

	// Drain the state machine error channel so read/parse errors are visible.
	// Without this, all readMessage() and dispatch errors are silently dropped.
	go func() {
		for er := range machine.ErrorReports() {
			if er.Conn != nil {
				log.Error("diameter: connection error",
					zap.Stringer("peer", er.Conn.RemoteAddr()),
					zap.Error(er.Error),
				)
			} else {
				log.Error("diameter: error", zap.Error(er.Error))
			}
		}
	}()

	return s, nil
}

// SendCLRByIMSI looks up the subscriber's serving MME and sends a
// Cancel-Location-Request with Cancellation-Type = SUBSCRIPTION_WITHDRAWAL.
func (s *Server) SendCLRByIMSI(ctx context.Context, imsi string) error {
	return s.s6a.SendCLRByIMSI(ctx, imsi, s6a.CancellationTypeSubscriptionWithdrawal)
}

// Start launches the Diameter listeners. TCP is always started; SCTP is
// started alongside it when hss.EnableSCTP is true in config.
// Both listeners share the same StateMachine (handler set).
// Returns the first fatal error from either listener.
func (s *Server) Start() error {
	addr := net.JoinHostPort(s.cfg.HSS.BindAddress, strconv.Itoa(s.cfg.HSS.BindPort))

	errCh := make(chan error, 2)

	// TCP listener (always on)
	go func() {
		s.log.Info("diameter: TCP listening", zap.String("addr", addr))
		srv := &diam.Server{Network: "tcp", Addr: addr, Handler: s.sm, Dict: dict.Default}
		errCh <- srv.ListenAndServe()
	}()

	// SCTP listener (optional — requires kernel SCTP module)
	if s.cfg.HSS.EnableSCTP {
		go func() {
			s.log.Info("diameter: SCTP listening", zap.String("addr", addr))
			srv := &diam.Server{Network: "sctp", Addr: addr, Handler: s.sm, Dict: dict.Default}
			if err := srv.ListenAndServe(); err != nil {
				s.log.Error("diameter: SCTP listener failed — is the SCTP kernel module loaded?",
					zap.Error(err))
				errCh <- err
			}
		}()
	}

	// Block until either listener dies.
	return <-errCh
}
