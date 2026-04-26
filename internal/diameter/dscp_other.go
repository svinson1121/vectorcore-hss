//go:build !linux

package diameter

import (
	"fmt"
	"net"
	"runtime"
)

func applyDiameterDSCP(conn net.Conn, dscp int) error {
	return applyDiameterDSCPToSocket(conn, dscp)
}

func applyDiameterDSCPToSocket(sock any, dscp int) error {
	if dscp == 0 {
		return nil
	}
	return fmt.Errorf("DSCP socket marking is not supported on %s", runtime.GOOS)
}
