package sbi

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/svinson1121/vectorcore-hss/internal/peertracker"
	"go.uber.org/zap"
)

type requestMetaKey struct{}

type RequestMeta struct {
	RequesterNFType string
	Requester       string
	ForwardedFor    string
	RemoteAddr      string
	ViaSCP          bool
}

func RequestMetaFromContext(ctx context.Context) RequestMeta {
	meta, _ := ctx.Value(requestMetaKey{}).(RequestMeta)
	return meta
}

func InboundMetadataMiddleware(forwarded *peertracker.Tracker, transport string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			meta := ParseRequestMeta(r)
			if forwarded != nil && meta.ViaSCP {
				forwarded.Add(peertracker.Peer{
					Name:       meta.DisplayName(),
					RemoteAddr: meta.DisplayRemoteAddr(),
					Transport:  meta.DisplayTransport(transport),
				})
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestMetaKey{}, meta)))
		})
	}
}

func ParseRequestMeta(r *http.Request) RequestMeta {
	if r == nil {
		return RequestMeta{}
	}
	meta := RequestMeta{
		Requester:  strings.TrimSpace(r.Header.Get("User-Agent")),
		RemoteAddr: strings.TrimSpace(r.RemoteAddr),
	}
	if t := strings.TrimSpace(r.Header.Get("3gpp-Sbi-Discovery-target-nf-type")); t != "" {
		meta.ViaSCP = true
	}
	if ff := firstForwardedFor(r.Header.Get("X-Forwarded-For")); ff != "" {
		meta.ForwardedFor = ff
		meta.ViaSCP = true
	}
	if requesterType := requesterNFType(meta.Requester); requesterType != "" {
		meta.RequesterNFType = requesterType
	}
	return meta
}

func (m RequestMeta) DisplayName() string {
	if m.Requester != "" {
		if m.ViaSCP {
			return m.Requester + " via SCP"
		}
		return m.Requester
	}
	if m.ViaSCP {
		return "SCP forwarded request"
	}
	return m.RemoteAddr
}

func (m RequestMeta) DisplayRemoteAddr() string {
	if m.ForwardedFor != "" {
		return m.ForwardedFor
	}
	return m.RemoteAddr
}

func (m RequestMeta) DisplayTransport(base string) string {
	if m.ViaSCP {
		return base + " via scp"
	}
	return base
}

func (m RequestMeta) LogFields() []zap.Field {
	fields := []zap.Field{
		zap.Bool("via_scp", m.ViaSCP),
	}
	if m.RequesterNFType != "" {
		fields = append(fields, zap.String("requester_nf_type", m.RequesterNFType))
	}
	if m.Requester != "" {
		fields = append(fields, zap.String("requester", m.Requester))
	}
	if addr := m.DisplayRemoteAddr(); addr != "" {
		fields = append(fields, zap.String("requester_remote_addr", addr))
	}
	return fields
}

func firstForwardedFor(raw string) string {
	if raw == "" {
		return ""
	}
	part := strings.TrimSpace(strings.Split(raw, ",")[0])
	if part == "" {
		return ""
	}
	if host, port, err := net.SplitHostPort(part); err == nil {
		return host + ":" + port
	}
	return part
}

func firstToken(raw, sep string) string {
	if raw == "" {
		return ""
	}
	part := strings.TrimSpace(strings.Split(raw, sep)[0])
	return part
}

func requesterNFType(userAgent string) string {
	if userAgent == "" {
		return ""
	}
	if strings.Contains(userAgent, "/") {
		return firstToken(userAgent, "/")
	}
	if strings.Contains(userAgent, "-") {
		return firstToken(userAgent, "-")
	}
	return strings.TrimSpace(userAgent)
}
