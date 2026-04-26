//go:build linux

package diameter

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type syscallConn interface {
	SyscallConn() (syscall.RawConn, error)
}

func applyDiameterDSCP(conn net.Conn, dscp int) error {
	return applyDiameterDSCPToSocket(conn, dscp)
}

func applyDiameterDSCPToSocket(sock any, dscp int) error {
	if dscp == 0 {
		return nil
	}
	if dscp < 0 || dscp > 63 {
		return fmt.Errorf("invalid DSCP value %d", dscp)
	}
	sc, ok := sock.(syscallConn)
	if !ok {
		fd, ok := legacySCTPFD(sock)
		if !ok {
			return fmt.Errorf("%T does not expose SyscallConn", sock)
		}
		return setDSCPOnFD(fd, dscpToTOS(dscp))
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return err
	}

	var setErr error
	controlErr := raw.Control(func(fd uintptr) {
		setErr = setDSCPOnFD(int(fd), dscpToTOS(dscp))
	})
	if controlErr != nil {
		return controlErr
	}
	return setErr
}

func setDSCPOnFD(fd int, tos int) error {
	err4 := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TOS, tos)
	err6 := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_TCLASS, tos)
	return firstMeaningfulSockoptError(err4, err6)
}

func firstMeaningfulSockoptError(err4, err6 error) error {
	if err4 == nil || err6 == nil {
		return nil
	}
	return errors.Join(err4, err6)
}

func legacySCTPFD(sock any) (int, bool) {
	v := reflect.ValueOf(sock)
	if !v.IsValid() {
		return 0, false
	}
	fd, ok := legacySCTPFDValue(v)
	if !ok || fd < 0 {
		return 0, false
	}
	return fd, true
}

func legacySCTPFDValue(v reflect.Value) (int, bool) {
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return 0, false
		}
		return legacySCTPFDValue(v.Elem())
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 0, false
		}
		return legacySCTPFDValue(v.Elem())
	}
	if v.Kind() != reflect.Struct {
		return 0, false
	}

	if f := v.FieldByName("_fd"); f.IsValid() && f.Kind() == reflect.Int32 {
		return int(*(*int32)(unsafe.Pointer(f.UnsafeAddr()))), true
	}
	if f := v.FieldByName("fd"); f.IsValid() && f.Kind() == reflect.Int {
		return *(*int)(unsafe.Pointer(f.UnsafeAddr())), true
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		if !t.Field(i).Anonymous {
			continue
		}
		if fd, ok := legacySCTPFDValue(v.Field(i)); ok {
			return fd, true
		}
	}
	return 0, false
}
