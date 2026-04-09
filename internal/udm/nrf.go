package udm

// nrf.go — NRF client: registration, heartbeat, and JWKS fetch.
//
// On startup the UDM registers itself with the NRF so that AUSF/AMF/SMF
// can discover our Nudm endpoints.  A heartbeat PATCH runs every
// heartBeatTimer seconds to maintain the registration.  The JWKS is fetched
// once at startup and cached for OAuth2 token validation.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"golang.org/x/net/http2"

	"github.com/svinson1121/vectorcore-hss/internal/peertracker"
)

const heartBeatTimer = 60 // seconds

type nfService struct {
	ServiceInstanceID string       `json:"serviceInstanceId"`
	ServiceName       string       `json:"serviceName"`
	Versions          []apiVersion `json:"versions"`
	Scheme            string       `json:"scheme"`
	NFServiceStatus   string       `json:"nfServiceStatus"`
	IPEndPoints       []ipEndPoint `json:"ipEndPoints"`
	APIPrefix         string       `json:"apiPrefix"`
}

type apiVersion struct {
	APIVersionInURI string `json:"apiVersionInUri"`
	APIFullVersion  string `json:"apiFullVersion"`
}

type ipEndPoint struct {
	IPv4Address string `json:"ipv4Address"`
	Port        int    `json:"port"`
}

type nfProfile struct {
	NFInstanceID   string      `json:"nfInstanceId"`
	NFInstanceName string      `json:"nfInstanceName"`
	NFType         string      `json:"nfType"`
	NFStatus       string      `json:"nfStatus"`
	HeartBeatTimer int         `json:"heartBeatTimer"`
	IPv4Addresses  []string    `json:"ipv4Addresses"`
	AllowedNFTypes []string    `json:"allowedNfTypes"`
	NFServices     []nfService `json:"nfServices"`
}

// startNRFRegistration registers the UDM with the NRF and starts the heartbeat.
// Runs in a separate goroutine; retries with backoff if the NRF is unreachable.
func (s *Server) startNRFRegistration() {
	if s.cfg.NRFAddress == "" {
		s.log.Info("udm: NRF address not configured, skipping registration")
		return
	}

	scheme := "http"
	if s.cfg.TLSCertFile != "" {
		scheme = "https"
	}

	ourIP := s.resolveOurIP()
	services := []nfService{
		makeService("nudm-ueau", scheme, ourIP, s.cfg.BindPort, []string{"v1", "v2"}),
		makeService("nudm-sdm", scheme, ourIP, s.cfg.BindPort, []string{"v1", "v2"}),
		makeService("nudm-uecm", scheme, ourIP, s.cfg.BindPort, []string{"v1"}),
		makeService("nudr-dr", scheme, ourIP, s.cfg.BindPort, []string{"v1", "v2"}),
	}

	profile := nfProfile{
		NFInstanceID:   s.cfg.NFInstanceID,
		NFInstanceName: "vectorcore-udm-udr",
		NFType:         "UDM",
		NFStatus:       "REGISTERED",
		HeartBeatTimer: heartBeatTimer,
		IPv4Addresses:  []string{ourIP},
		AllowedNFTypes: []string{"AUSF", "AMF", "SMF", "PCF", "NEF"},
		NFServices:     services,
	}

	client := s.nrfClient()

	// Retry registration with exponential backoff.
	backoff := 2 * time.Second
	for {
		if err := s.nrfRegister(client, profile); err != nil {
			s.log.Warn("udm: NRF registration failed, retrying",
				zap.Error(err), zap.Duration("backoff", backoff))
			time.Sleep(backoff)
			if backoff < 60*time.Second {
				backoff *= 2
			}
			continue
		}
		s.log.Info("udm: registered with NRF",
			zap.String("instance_id", s.cfg.NFInstanceID),
			zap.String("nrf", s.cfg.NRFAddress),
		)
		break
	}

	// Fetch JWKS for OAuth2 token validation.
	go s.fetchJWKS(client)

	// Start heartbeat loop.
	go s.heartbeat(client, profile.NFInstanceID)
}

func (s *Server) nrfRegister(client *http.Client, profile nfProfile) error {
	body, _ := json.Marshal(profile)
	url := fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/%s", s.cfg.NRFAddress, profile.NFInstanceID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		s.removeSBIPeer("NRF", s.cfg.NRFAddress)
		return fmt.Errorf("NRF returned %d: %s", resp.StatusCode, b)
	}
	s.addSBIPeer("NRF", s.cfg.NRFAddress)
	return nil
}

func (s *Server) heartbeat(client *http.Client, instanceID string) {
	t := time.NewTicker(heartBeatTimer * time.Second)
	defer t.Stop()
	patch := []map[string]interface{}{
		{"op": "replace", "path": "/nfStatus", "value": "REGISTERED"},
	}
	body, _ := json.Marshal(patch)
	url := fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/%s", s.cfg.NRFAddress, instanceID)
	for range t.C {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, url, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json-patch+json")
		resp, err := client.Do(req)
		if err != nil {
			s.removeSBIPeer("NRF", s.cfg.NRFAddress)
			s.log.Warn("udm: NRF heartbeat failed", zap.Error(err))
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			s.removeSBIPeer("NRF", s.cfg.NRFAddress)
			s.log.Warn("udm: NRF heartbeat returned error", zap.Int("status", resp.StatusCode))
			continue
		}
		s.addSBIPeer("NRF", s.cfg.NRFAddress)
	}
}

func (s *Server) fetchJWKS(client *http.Client) {
	url := fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances?nf-type=NRF", s.cfg.NRFAddress)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		s.removeSBIPeer("NRF", s.cfg.NRFAddress)
		s.log.Warn("udm: JWKS fetch failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		s.removeSBIPeer("NRF", s.cfg.NRFAddress)
		s.log.Warn("udm: JWKS fetch returned error", zap.Int("status", resp.StatusCode))
		return
	}
	raw, _ := io.ReadAll(resp.Body)
	if len(raw) > 0 {
		s.addSBIPeer("NRF", s.cfg.NRFAddress)
		s.jwks.set(raw)
		s.log.Info("udm: JWKS cached from NRF")
	}
}

func (s *Server) addSBIPeer(name, rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return
	}
	transport := "h2c"
	if u.Scheme == "https" {
		transport = "https/h2"
	}
	s.Peers().Add(peertracker.Peer{
		Name:       name,
		RemoteAddr: u.Host,
		Transport:  transport,
	})
}

func (s *Server) removeSBIPeer(name, rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return
	}
	s.Peers().Remove(u.Host)
}

// nrfClient returns an HTTP/2 client appropriate for the NRF transport.
func (s *Server) nrfClient() *http.Client {
	tr := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}

func (s *Server) resolveOurIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func makeService(name, scheme, ip string, port int, versions []string) nfService {
	var vers []apiVersion
	for _, v := range versions {
		vers = append(vers, apiVersion{APIVersionInURI: v, APIFullVersion: v + ".0.0"})
	}
	return nfService{
		ServiceInstanceID: name,
		ServiceName:       name,
		Versions:          vers,
		Scheme:            scheme,
		NFServiceStatus:   "REGISTERED",
		IPEndPoints:       []ipEndPoint{{IPv4Address: ip, Port: port}},
		APIPrefix:         name,
	}
}
