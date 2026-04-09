package gsup

// server.go -- IPA/GSUP TCP server.
//
// Connection lifecycle per Osmocom IPA protocol:
//   1. Peer connects via TCP.
//   2. HSS sends ID_GET (CCM) requesting peer identity.
//   3. Peer responds with ID_RESP containing its unit-name.
//   4. Peer sends GSUP requests (AIR, ULR, PUR, ...).
//   5. HSS handles requests and sends responses.
//   6. Keepalive: peer sends PING, HSS responds with PONG.

import (
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/peertracker"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// Server is the GSUP/IPA TCP server.
type Server struct {
	cfg   config.GSUPConfig
	store repository.Repository
	log   *zap.Logger
	pub   geored.TypedPublisher
	pt    *peertracker.Tracker
}

// New creates a new GSUP server.
func New(cfg config.GSUPConfig, _ config.HSSConfig, store repository.Repository, log *zap.Logger) *Server {
	return &Server{cfg: cfg, store: store, log: log, pub: geored.NoopTypedPublisher{}, pt: peertracker.New()}
}

// WithGeored attaches a GeoRed publisher to the GSUP server.
func (s *Server) WithGeored(pub geored.TypedPublisher) *Server {
	s.pub = pub
	return s
}

// Peers returns the live GSUP peer tracker.
func (s *Server) Peers() *peertracker.Tracker {
	if s.pt == nil {
		s.pt = peertracker.New()
	}
	return s.pt
}

// Start listens for inbound IPA/GSUP connections.
// Blocks until a fatal listen error; each peer is handled in its own goroutine.
func (s *Server) Start() error {
	addr := net.JoinHostPort(s.cfg.BindAddress, itoa(s.cfg.BindPort))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.log.Info("gsup: listening", zap.String("addr", addr))

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.log.Error("gsup: accept error", zap.Error(err))
			continue
		}
		go s.handleConn(conn)
	}
}

// handleConn manages the full lifecycle of one peer connection.
func (s *Server) handleConn(conn net.Conn) {
	remote := conn.RemoteAddr().String()
	s.log.Info("gsup: peer connected", zap.String("peer", remote))
	s.Peers().Add(peertracker.Peer{Name: remote, RemoteAddr: remote, Transport: "tcp"})
	defer func() {
		conn.Close()
		s.Peers().Remove(remote)
		s.log.Info("gsup: peer disconnected", zap.String("peer", remote))
	}()

	// Send CCM ID_GET so OsmoMSC's GSUP client completes the CCM handshake
	// and transitions to "ready" state. Without this, OsmoMSC never sends
	// GSUP ULR/AIR even though IPA PING/PONG continues to work.
	if err := ipaWriteIDGet(conn); err != nil {
		s.log.Error("gsup: ID_GET write failed", zap.String("peer", remote), zap.Error(err))
		return
	}

	// OsmoMSC also sends ID_RESP proactively on connect; we extract the
	// unit-name when it arrives. Fall back to the remote address if not.
	peerName := remote
	// peerProto tracks which IPA OSMO proto byte the peer uses (0xFE current
	// or 0xEE legacy). Responses must use the same byte or old libosmocom
	// peers silently discard them. Default to current; updated on first frame.
	peerProto := byte(ipaProtoOSMO)

	for {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		msg, err := ipaRead(conn)
		if err != nil {
			if !isEOF(err) {
				s.log.Error("gsup: read error", zap.String("peer", peerName), zap.Error(err))
			}
			return
		}

		switch msg.proto {
		case ipaProtoCCM:
			peerName = s.handleCCM(conn, remote, peerName, msg)

		case ipaProtoOSMO, ipaProtoOSMOLegacy:
			peerProto = msg.proto // echo the peer's own proto byte in all responses
			switch msg.ext {
			case ipaOSMOPing:
				if err := ipaWriteOSMOPong(conn); err != nil {
					s.log.Error("gsup: PONG write failed", zap.String("peer", peerName), zap.Error(err))
				}
			case ipaOSMOPong:
				// ignore — we don't send pings, so this shouldn't arrive
			case ipaExtGSUP:
				gsupMsg, err := Decode(msg.payload)
				if err != nil {
					s.log.Warn("gsup: decode error",
						zap.String("peer", peerName),
						zap.Error(err),
					)
					continue
				}
				s.handleMessage(conn, peerName, peerProto, gsupMsg)
			default:
				s.log.Debug("gsup: ignoring unknown OSMO ext",
					zap.String("peer", peerName),
					zap.Uint8("ext", msg.ext),
				)
			}

		default:
			s.log.Warn("gsup: ignoring non-GSUP IPA frame",
				zap.String("peer", peerName),
				zap.Uint8("proto", msg.proto),
			)
		}
	}
}

// handleCCM processes Common Control Messages (PING, ID_RESP).
// Returns the (possibly updated) peer name.
func (s *Server) handleCCM(conn net.Conn, remoteAddr, peerName string, msg *ipaMsg) string {
	if len(msg.payload) == 0 {
		return peerName
	}
	switch msg.payload[0] {
	case ccmMsgPING:
		if _, err := conn.Write([]byte{0x00, 0x01, ipaProtoCCM, ccmMsgPONG}); err != nil {
			s.log.Error("gsup: CCM PONG write failed", zap.String("peer", peerName), zap.Error(err))
		}
		return peerName
	case ccmMsgIDResp:
		if name := parseIDResp(msg.payload); name != "" {
			s.log.Info("gsup: peer identified",
				zap.String("addr", peerName),
				zap.String("name", name),
			)
			s.Peers().Rename(remoteAddr, name)
			peerName = name
		}
		// ID_ACK completes the IPA CCM handshake; OsmoMSC's GSUP client
		// will not send ULR/AIR until it receives this acknowledgement.
		if err := ipaWriteIDACK(conn); err != nil {
			s.log.Error("gsup: ID_ACK write failed", zap.String("peer", peerName), zap.Error(err))
		}
		return peerName
	}
	return peerName
}

func isEOF(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return s == "EOF" ||
		contains(s, "connection reset") ||
		contains(s, "use of closed") ||
		contains(s, "i/o timeout")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 6)
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
