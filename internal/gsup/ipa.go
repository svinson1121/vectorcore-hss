package gsup

// IPA (IP.access) protocol framing used by Osmocom as the transport layer
// for GSUP over TCP.
//
// Wire format of each IPA message:
//
//	[2 bytes] payload length (big-endian, NOT including the 3-byte header)
//	[1 byte]  protocol ID
//	[N bytes] payload
//
// Protocol IDs relevant to GSUP:
//
//	0xFE  OSMO  -- Osmocom extension; carries GSUP sub-protocol
//	0xFE + extension byte 0x05 = GSUP
//
// The sub-protocol byte immediately follows the protocol byte for OSMO messages,
// so the effective header for a GSUP message is 4 bytes:
//
//	len(2) | proto=0xFE | ext=0x05 | gsup-payload...
//
// CCM (Common Control Messages, proto=0xFF) are used for peer identity exchange
// (ID_GET / ID_RESP) and keepalive (PING / PONG).

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const (
	ipaProtoCCM      = 0xFF // Common Control Messages (ID_GET/ID_RESP)
	ipaProtoOSMO     = 0xFE // Osmocom extension (current libosmocore)
	ipaProtoOSMOLegacy = 0xEE // Osmocom extension (old libosmocore, same format)
	ipaExtGSUP   = 0x05 // GSUP sub-protocol under OSMO

	// OSMO-level keepalive (proto=0xFE, NOT CCM).
	// libosmocore uses these rather than CCM 0xFF for ping/pong.
	ipaOSMOPing = 0x00
	ipaOSMOPong = 0x01

	// CCM message types (proto=0xFF)
	ccmMsgPING   = 0x00
	ccmMsgPONG   = 0x01
	ccmMsgIDGet  = 0x04
	ccmMsgIDResp = 0x05
	ccmMsgIDACK  = 0x06

	// CCM identity tags sent in ID_RESP
	ipaTagSerial   = 0x00
	ipaTagUnitName = 0x01
	ipaTagLocation = 0x02
	ipaTagUnitType = 0x03
	ipaTagEquipVer = 0x04
	ipaTagSwVer    = 0x05
	ipaTagIPAddr   = 0x06
	ipaTagMACAddr  = 0x07
	ipaTagUnitID   = 0x08
)

// ipaMsg is a decoded IPA frame.
type ipaMsg struct {
	proto   byte
	ext     byte // only valid when proto == ipaProtoOSMO
	payload []byte
}

// ipaRead reads and decodes one IPA frame from conn.
func ipaRead(conn net.Conn) (*ipaMsg, error) {
	hdr := make([]byte, 3)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint16(hdr[0:2])
	proto := hdr[2]

	if length == 0 {
		return &ipaMsg{proto: proto}, nil
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}

	msg := &ipaMsg{proto: proto}
	if proto == ipaProtoOSMO || proto == ipaProtoOSMOLegacy {
		if len(body) < 1 {
			return nil, fmt.Errorf("ipa: OSMO frame too short")
		}
		msg.ext = body[0]
		msg.payload = body[1:]
	} else {
		msg.payload = body
	}
	return msg, nil
}

// ipaWriteGSUP writes a GSUP payload wrapped in IPA framing to conn.
// proto must match the proto byte the peer used when sending its request
// (ipaProtoOSMO=0xFE or ipaProtoOSMOLegacy=0xEE); old libosmocom peers
// only accept responses whose proto byte matches their own.
func ipaWriteGSUP(conn net.Conn, proto byte, payload []byte) error {
	// Header: 2-byte length + proto + ext(0x05) + payload
	// Length field covers ext byte + payload.
	totalBody := 1 + len(payload) // ext byte + payload
	frame := make([]byte, 3+totalBody)
	binary.BigEndian.PutUint16(frame[0:2], uint16(totalBody))
	frame[2] = proto
	frame[3] = ipaExtGSUP
	copy(frame[4:], payload)
	_, err := conn.Write(frame)
	return err
}

// ipaWriteOSMOPong sends an IPA PONG using the OSMO sub-protocol (proto=0xFE,
// ext=0x01). libosmocom uses this rather than the CCM PING/PONG for keepalive.
func ipaWriteOSMOPong(conn net.Conn) error {
	frame := []byte{0x00, 0x01, ipaProtoOSMO, ipaOSMOPong}
	_, err := conn.Write(frame)
	return err
}

// ipaWriteIDGet sends a CCM ID_GET request asking the peer to identify itself.
// The request includes the UNIT_NAME tag so OsmoMSC's CCM parser accepts it
// and its GSUP client transitions to "ready" state, enabling GSUP ULR/AIR.
//
// Frame: [len=3] [proto=0xFF] [ID_GET=0x04] [tagLen=0x01] [tag=UNIT_NAME=0x01]
func ipaWriteIDGet(conn net.Conn) error {
	frame := []byte{0x00, 0x03, ipaProtoCCM, ccmMsgIDGet, 0x01, ipaTagUnitName}
	_, err := conn.Write(frame)
	return err
}

// ipaWriteIDACK sends a CCM ID_ACK completing the IPA CCM handshake.
// OsmoMSC's GSUP client requires this acknowledgement before it will
// send any GSUP messages (ULR, AIR, etc.).
//
// Frame: [len=1] [proto=0xFF] [ID_ACK=0x06]
func ipaWriteIDACK(conn net.Conn) error {
	frame := []byte{0x00, 0x01, ipaProtoCCM, ccmMsgIDACK}
	_, err := conn.Write(frame)
	return err
}

// parseIDResp extracts the unit-name tag from an ID_RESP payload.
// Returns the peer name (unit-name tag) if present.
func parseIDResp(payload []byte) string {
	i := 1 // skip the msg type byte
	for i < len(payload) {
		if i+1 >= len(payload) {
			break
		}
		tagLen := int(payload[i])
		tag := payload[i+1]
		i += 2
		if i+tagLen-1 > len(payload) {
			break
		}
		val := payload[i : i+tagLen-1]
		i += tagLen - 1
		if tag == ipaTagUnitName {
			return string(val)
		}
	}
	return ""
}
