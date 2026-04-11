package sbi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/peertracker"
)

const HeartbeatTimer = 60 // seconds

// PLMN identifies a 3GPP Public Land Mobile Network by MCC and MNC.
type PLMN struct {
	MCC string `json:"mcc"`
	MNC string `json:"mnc"`
}

type NFService struct {
	ServiceInstanceID string       `json:"serviceInstanceId"`
	ServiceName       string       `json:"serviceName"`
	Versions          []APIVersion `json:"versions"`
	Scheme            string       `json:"scheme"`
	NFServiceStatus   string       `json:"nfServiceStatus"`
	IPEndPoints       []IPEndPoint `json:"ipEndPoints"`
	APIPrefix         string       `json:"apiPrefix"`
}

type APIVersion struct {
	APIVersionInURI string `json:"apiVersionInUri"`
	APIFullVersion  string `json:"apiFullVersion"`
}

type IPEndPoint struct {
	IPv4Address string `json:"ipv4Address"`
	Port        int    `json:"port"`
}

type NFProfile struct {
	NFInstanceID   string      `json:"nfInstanceId"`
	NFInstanceName string      `json:"nfInstanceName"`
	NFType         string      `json:"nfType"`
	NFStatus       string      `json:"nfStatus"`
	HeartBeatTimer int         `json:"heartBeatTimer"`
	PLMNList       []PLMN      `json:"plmnList,omitempty"`
	IPv4Addresses  []string    `json:"ipv4Addresses"`
	AllowedNFTypes []string    `json:"allowedNfTypes"`
	NFServices     []NFService `json:"nfServices"`
}

// PLMNListFromConfig returns a single-entry PLMN list if both mcc and mnc are
// non-empty, or nil otherwise (causing the NRF to default to its own PLMN —
// which is wrong for home-PLMN UDM/PCF discovery).
func PLMNListFromConfig(mcc, mnc string) []PLMN {
	if mcc == "" || mnc == "" {
		return nil
	}
	return []PLMN{{MCC: mcc, MNC: mnc}}
}

func MakeService(name, scheme, ip string, port int, versions []string) NFService {
	var vers []APIVersion
	for _, v := range versions {
		vers = append(vers, APIVersion{APIVersionInURI: v, APIFullVersion: v + ".0.0"})
	}
	return NFService{
		ServiceInstanceID: name,
		ServiceName:       name,
		Versions:          vers,
		Scheme:            scheme,
		NFServiceStatus:   "REGISTERED",
		IPEndPoints:       []IPEndPoint{{IPv4Address: ip, Port: port}},
		APIPrefix:         name,
	}
}

type NRFRegistrar struct {
	nfName     string
	nrfAddress string
	instanceID string
	profile    NFProfile
	log        *zap.Logger
	peers      *peertracker.Tracker
	client     *Client
}

func NewNRFRegistrar(nfName, nrfAddress, instanceID string, profile NFProfile, log *zap.Logger, peers *peertracker.Tracker, clientCfg config.SBIClientConfig) *NRFRegistrar {
	return &NRFRegistrar{
		nfName:     nfName,
		nrfAddress: nrfAddress,
		instanceID: instanceID,
		profile:    profile,
		log:        log,
		peers:      peers,
		client:     NewClient(clientCfg),
	}
}

func (r *NRFRegistrar) Client() *Client { return r.client }

func (r *NRFRegistrar) Start() {
	if r.nrfAddress == "" {
		r.log.Info(fmt.Sprintf("%s: NRF address not configured, skipping registration", r.nfName))
		return
	}

	holdoff := r.client.cfg.ReconnectHoldoffTime
	if holdoff <= 0 {
		holdoff = 2 * time.Second
	}
	for {
		if err := r.Register(); err != nil {
			r.log.Warn(fmt.Sprintf("%s: NRF registration failed, retrying", r.nfName),
				zap.Error(err), zap.Duration("holdoff", holdoff))
			time.Sleep(holdoff)
			continue
		}
		r.log.Info(fmt.Sprintf("%s: registered with NRF", r.nfName),
			zap.String("instance_id", r.instanceID),
			zap.String("nrf", r.nrfAddress),
		)
		break
	}

	go r.Heartbeat()
}

func (r *NRFRegistrar) Register() error {
	body, _ := json.Marshal(r.profile)
	endpoint := fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/%s", r.nrfAddress, r.profile.NFInstanceID)
	req, err := r.client.NewRequestWithOptions(context.Background(), "PUT", endpoint, bytes.NewReader(body), RequestOptions{
		RequesterNFType:       r.profile.NFType,
		RequesterNFInstanceID: r.profile.NFInstanceID,
		TargetNFType:          "NRF",
		TargetServiceName:     "nnrf-nfm",
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", FormatRequesterUserAgent(r.profile.NFType, r.profile.NFInstanceID))
	resp, err := r.client.Do(req)
	if err != nil {
		r.removePeer("NRF", r.nrfAddress)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		r.removePeer("NRF", r.nrfAddress)
		return fmt.Errorf("NRF returned %d: %s", resp.StatusCode, b)
	}
	r.addPeer("NRF", r.nrfAddress)
	return nil
}

func (r *NRFRegistrar) Heartbeat() {
	t := time.NewTicker(HeartbeatTimer * time.Second)
	defer t.Stop()
	patch := []map[string]interface{}{
		{"op": "replace", "path": "/nfStatus", "value": "REGISTERED"},
	}
	body, _ := json.Marshal(patch)
	endpoint := fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/%s", r.nrfAddress, r.profile.NFInstanceID)
	for range t.C {
		r.heartbeatOnce(context.Background(), endpoint, body)
	}
}

func (r *NRFRegistrar) heartbeatOnce(ctx context.Context, endpoint string, body []byte) {
	req, err := r.client.NewRequestWithOptions(ctx, "PATCH", endpoint, bytes.NewReader(body), RequestOptions{
		RequesterNFType:       r.profile.NFType,
		RequesterNFInstanceID: r.profile.NFInstanceID,
		TargetNFType:          "NRF",
		TargetServiceName:     "nnrf-nfm",
	})
	if err != nil {
		r.removePeer("NRF", r.nrfAddress)
		r.log.Warn(fmt.Sprintf("%s: NRF heartbeat request build failed", r.nfName), zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json-patch+json")
	req.Header.Set("User-Agent", FormatRequesterUserAgent(r.profile.NFType, r.profile.NFInstanceID))
	resp, err := r.client.Do(req)
	if err != nil {
		r.removePeer("NRF", r.nrfAddress)
		r.log.Warn(fmt.Sprintf("%s: NRF heartbeat failed", r.nfName), zap.Error(err))
		return
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		r.removePeer("NRF", r.nrfAddress)
		r.log.Warn(fmt.Sprintf("%s: NRF heartbeat lost registration, re-registering", r.nfName))
		if err := r.Register(); err != nil {
			r.log.Warn(fmt.Sprintf("%s: NRF re-registration failed", r.nfName), zap.Error(err))
		}
		return
	}
	if resp.StatusCode >= 400 {
		r.removePeer("NRF", r.nrfAddress)
		r.log.Warn(fmt.Sprintf("%s: NRF heartbeat returned error", r.nfName), zap.Int("status", resp.StatusCode))
		return
	}
	r.addPeer("NRF", r.nrfAddress)
}

func (r *NRFRegistrar) addPeer(name, rawURL string) {
	if r.peers == nil {
		return
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return
	}
	transport := "h2c"
	if u.Scheme == "https" {
		transport = "https/h2"
	}
	r.peers.Add(peertracker.Peer{Name: name, RemoteAddr: u.Host, Transport: transport})
}

func (r *NRFRegistrar) removePeer(_ string, rawURL string) {
	if r.peers == nil {
		return
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return
	}
	r.peers.Remove(u.Host)
}
