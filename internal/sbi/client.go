package sbi

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"golang.org/x/net/http2"
)

const (
	RoutingModeDirect = "direct"
	RoutingModeSCP    = "scp"
)

// Client wraps outbound SBI HTTP behavior and centralizes route selection.
type Client struct {
	cfg        config.SBIClientConfig
	httpClient *http.Client
}

type RequestOptions struct {
	RequesterNFType       string
	RequesterNFInstanceID string
	TargetNFType          string
	TargetServiceName     string
}

func NewClient(cfg config.SBIClientConfig) *Client {
	mode := strings.TrimSpace(strings.ToLower(cfg.Mode))
	if mode == "" {
		mode = RoutingModeDirect
	}
	cfg.Mode = mode
	return &Client{
		cfg:        cfg,
		httpClient: newHTTP2Client(),
	}
}

func (c *Client) HTTPClient() *http.Client { return c.httpClient }

func (c *Client) NewRequest(ctx context.Context, method, targetURL string, body io.Reader) (*http.Request, error) {
	return c.NewRequestWithOptions(ctx, method, targetURL, body, RequestOptions{})
}

func (c *Client) NewRequestWithOptions(ctx context.Context, method, targetURL string, body io.Reader, opts RequestOptions) (*http.Request, error) {
	switch c.cfg.Mode {
	case RoutingModeDirect:
		req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
		if err != nil {
			return nil, err
		}
		applyRequesterHeaders(req, opts)
		return req, nil
	case RoutingModeSCP:
		if strings.TrimSpace(c.cfg.SCPAddress) == "" {
			return nil, fmt.Errorf("sbi: routing mode scp requires sbi_client.scp_address")
		}
		return c.newSCPRequest(ctx, method, targetURL, body, opts)
	default:
		return nil, fmt.Errorf("sbi: unsupported routing mode %q", c.cfg.Mode)
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

func newHTTP2Client() *http.Client {
	tr := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}

func (c *Client) newSCPRequest(ctx context.Context, method, targetURL string, body io.Reader, opts RequestOptions) (*http.Request, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("sbi: parse target URL: %w", err)
	}
	scp, err := url.Parse(c.cfg.SCPAddress)
	if err != nil {
		return nil, fmt.Errorf("sbi: parse SCP address: %w", err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("sbi: target URL must be absolute")
	}
	if scp.Scheme == "" || scp.Host == "" {
		return nil, fmt.Errorf("sbi: scp_address must be absolute")
	}

	routed := *scp
	routed.Path = joinURLPath(scp.Path, target.Path)
	routed.RawPath = ""
	routed.RawQuery = target.RawQuery
	routed.Fragment = ""

	req, err := http.NewRequestWithContext(ctx, method, routed.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("3gpp-Sbi-Target-apiRoot", target.Scheme+"://"+target.Host)
	if opts.TargetNFType != "" {
		req.Header.Set("3gpp-Sbi-Discovery-target-nf-type", opts.TargetNFType)
	}
	if opts.TargetServiceName != "" {
		req.Header.Set("3gpp-Sbi-Discovery-service-names", opts.TargetServiceName)
	}
	applyRequesterHeaders(req, opts)
	return req, nil
}

func applyRequesterHeaders(req *http.Request, opts RequestOptions) {
	if req == nil {
		return
	}
	if ua := FormatRequesterUserAgent(opts.RequesterNFType, opts.RequesterNFInstanceID); ua != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", ua)
	}
}

func FormatRequesterUserAgent(nfType, nfInstanceID string) string {
	if nfType == "" {
		return ""
	}
	if nfInstanceID == "" {
		return nfType
	}
	return nfType + "-" + nfInstanceID
}

func joinURLPath(basePath, targetPath string) string {
	if basePath == "" || basePath == "/" {
		if targetPath == "" {
			return "/"
		}
		return targetPath
	}
	if targetPath == "" || targetPath == "/" {
		return basePath
	}
	return path.Join(basePath, targetPath)
}
