package support

// Minimal RFC 6455 WebSocket server primitives (server side only, text frames,
// no extensions, no fragmentation). It is intentionally tiny and has no
// third-party dependency: the connection is taken over from the Fiber/fasthttp
// request via Hijack and then driven by the Hub in hub.go.

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// WebSocket opcodes we care about.
const (
	opText  byte = 0x1
	opClose byte = 0x8
	opPing  byte = 0x9
	opPong  byte = 0xA
)

// maxWSFrame caps an inbound frame so a client cannot make us allocate freely.
const maxWSFrame = 64 * 1024

var errWSUnsupported = errors.New("websocket: unsupported frame")

// wsAccept computes the Sec-WebSocket-Accept value for a client key.
func wsAccept(key string) string {
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// wsConn wraps a hijacked connection with serialized frame writes.
type wsConn struct {
	conn net.Conn
	mu   sync.Mutex
}

// writeFrame sends a single unmasked, final frame (server → client).
func (w *wsConn) writeFrame(op byte, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	hdr := make([]byte, 0, 10)
	hdr = append(hdr, 0x80|op)
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n <= 0xFFFF:
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 127)
		var l [8]byte
		binary.BigEndian.PutUint64(l[:], uint64(n))
		hdr = append(hdr, l[:]...)
	}
	_ = w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := w.conn.Write(hdr); err != nil {
		return err
	}
	if n > 0 {
		if _, err := w.conn.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// readFrame reads one client → server frame. Client frames must be masked
// (RFC 6455 §5.1); fragmented messages are not supported.
func readFrame(r *bufio.Reader) (op byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	fin := h[0]&0x80 != 0
	op = h[0] & 0x0F
	masked := h[1]&0x80 != 0
	length := uint64(h[1] & 0x7F)

	if !fin || op == 0x0 || !masked {
		return 0, nil, errWSUnsupported
	}
	switch length {
	case 126:
		var b [2]byte
		if _, err = io.ReadFull(r, b[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err = io.ReadFull(r, b[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(b[:])
	}
	if length > maxWSFrame {
		return 0, nil, errWSUnsupported
	}
	var mask [4]byte
	if _, err = io.ReadFull(r, mask[:]); err != nil {
		return 0, nil, err
	}
	payload = make([]byte, length)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return op, payload, nil
}
