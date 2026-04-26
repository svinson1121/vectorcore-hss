package diameter

import (
	"net"

	"go.uber.org/zap"
)

type dscpListener struct {
	net.Listener
	dscp int
	log  *zap.Logger
}

func (l dscpListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if err := applyDiameterDSCP(conn, l.dscp); err != nil {
		l.log.Warn("diameter: failed to apply DSCP marking",
			zap.String("transport", conn.RemoteAddr().Network()),
			zap.String("peer", conn.RemoteAddr().String()),
			zap.Int("dscp", l.dscp),
			zap.Error(err),
		)
	}
	return conn, nil
}

func maybeWrapDSCPListener(l net.Listener, dscp int, log *zap.Logger) net.Listener {
	if dscp == 0 {
		return l
	}
	return dscpListener{Listener: l, dscp: dscp, log: log}
}

func dscpToTOS(dscp int) int {
	return dscp << 2
}
