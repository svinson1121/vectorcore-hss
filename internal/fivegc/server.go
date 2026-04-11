package fivegc

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/pcf"
	"github.com/svinson1121/vectorcore-hss/internal/peertracker"
	"github.com/svinson1121/vectorcore-hss/internal/sbi"
	"github.com/svinson1121/vectorcore-hss/internal/udm"
)

type Server struct {
	bindAddress string
	bindPort    int
	tlsCertFile string
	tlsKeyFile  string
	scpAddress  string
	log         *zap.Logger
	udm         *udm.Server
	pcf         *pcf.Server
	pt          *peertracker.Tracker
	fpt         *peertracker.Tracker
}

func New(udmSrv *udm.Server, pcfSrv *pcf.Server, log *zap.Logger) *Server {
	s := &Server{
		log: log,
		udm: udmSrv,
		pcf: pcfSrv,
		pt:  peertracker.New(),
		fpt: peertracker.NewWithMaxAge(2 * time.Minute),
	}
	if udmSrv != nil {
		udmCfg := udmSrv.Config()
		s.bindAddress = udmCfg.BindAddress
		s.bindPort = udmCfg.BindPort
		s.tlsCertFile = udmCfg.TLSCertFile
		s.tlsKeyFile = udmCfg.TLSKeyFile
		s.scpAddress = udmCfg.SBIClient.SCPAddress
		udmSrv.UsePeerTrackers(s.pt, s.fpt)
	}
	if pcfSrv != nil {
		pcfCfg := pcfSrv.Config()
		if s.bindAddress == "" {
			s.bindAddress = pcfCfg.BindAddress
			s.bindPort = pcfCfg.BindPort
			s.tlsCertFile = pcfCfg.TLSCertFile
			s.tlsKeyFile = pcfCfg.TLSKeyFile
		}
		if s.scpAddress == "" {
			s.scpAddress = pcfCfg.SBIClient.SCPAddress
		}
		pcfSrv.UsePeerTrackers(s.pt, s.fpt)
	}
	return s
}

func Compatible(udmCfg config.UDMConfig, pcfCfg config.PCFConfig) bool {
	return udmCfg.BindAddress == pcfCfg.BindAddress &&
		udmCfg.BindPort == pcfCfg.BindPort &&
		udmCfg.TLSCertFile == pcfCfg.TLSCertFile &&
		udmCfg.TLSKeyFile == pcfCfg.TLSKeyFile
}

func (s *Server) Start() error {
	r := chi.NewRouter()
	if s.udm != nil {
		s.udm.MountRoutes(r)
	}
	if s.pcf != nil {
		s.pcf.MountRoutes(r)
	}

	addr := net.JoinHostPort(s.bindAddress, fmt.Sprintf("%d", s.bindPort))
	s.log.Info("5gc: shared SBI listening", zap.String("addr", addr))

	if s.udm != nil {
		go s.udm.StartNRFRegistration()
	}
	if s.pcf != nil {
		go s.pcf.StartNRFRegistration()
	}

	if s.tlsCertFile != "" && s.tlsKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.tlsCertFile, s.tlsKeyFile)
		if err != nil {
			return fmt.Errorf("5gc: load TLS cert: %w", err)
		}
		srv := &http.Server{
			Addr:    addr,
			Handler: sbi.InboundMetadataMiddleware(s.fpt, "https/h2")(r),
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				NextProtos:   []string{"h2"},
			},
			ConnState: s.connState("https/h2"),
		}
		http2.ConfigureServer(srv, nil)
		return srv.ListenAndServeTLS("", "")
	}

	srv := &http.Server{
		Addr:      addr,
		Handler:   h2c.NewHandler(sbi.InboundMetadataMiddleware(s.fpt, "h2c")(r), &http2.Server{}),
		ConnState: s.connState("h2c"),
	}
	return srv.ListenAndServe()
}

func (s *Server) connState(transport string) func(net.Conn, http.ConnState) {
	return func(conn net.Conn, state http.ConnState) {
		remote := conn.RemoteAddr().String()
		switch state {
		case http.StateNew, http.StateActive, http.StateIdle:
			s.pt.Add(peertracker.Peer{
				Name:       sbi.PeerDisplayName(remote, s.scpAddress),
				RemoteAddr: remote,
				Transport:  transport,
			})
		case http.StateHijacked, http.StateClosed:
			s.pt.Remove(remote)
		}
	}
}
