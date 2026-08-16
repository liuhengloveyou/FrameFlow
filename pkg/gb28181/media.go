package gb28181

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
)

// MediaReceiver accepts PS-over-RTP from a GB28181 device.
type MediaReceiver struct {
	Transport string // udp | tcp-passive | tcp-active
	LocalIP   string
	Port      uint16

	udpConn *net.UDPConn
	tcpLn   *net.TCPListener
	tcpConn net.Conn

	mu     sync.Mutex
	closed bool

	rtpCount    atomic.Uint64
	rtpBytes    atomic.Uint64
	rtpBad      atomic.Uint64
	loggedFirst atomic.Bool

	// OnPayload is called with each RTP payload (PS fragment).
	OnPayload func(payload []byte, ts uint32)
	// Label for logs (device/channel).
	Label string
}

func (m *MediaReceiver) Listen() error {
	switch NormalizeMediaTransport(m.Transport) {
	case TransportTCPPassive:
		return m.listenTCP()
	case TransportTCPActive:
		// TCP active: dial after INVITE answer; nothing to listen here.
		return nil
	default:
		return m.listenUDP()
	}
}

// DialTCP connects to the device media port (TCP-ACTIVE / setup:active).
func (m *MediaReceiver) DialTCP(ip string, port int) error {
	if port <= 0 || ip == "" {
		return fmt.Errorf("gb28181: invalid tcp-active remote %s:%d", ip, port)
	}
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	log.Info().
		Str("label", m.Label).
		Str("transport", "tcp-active").
		Str("remote", conn.RemoteAddr().String()).
		Msg("[gb28181] media: tcp dialed")
	m.mu.Lock()
	m.tcpConn = conn
	m.mu.Unlock()
	go m.readTCP(conn)
	return nil
}

func (m *MediaReceiver) listenUDP() error {
	addr := &net.UDPAddr{IP: net.ParseIP(m.LocalIP), Port: int(m.Port)}
	if addr.IP == nil {
		addr.IP = net.IPv4zero
	}
	c, err := net.ListenUDP("udp", addr)
	if err != nil {
		// fallback any interface
		c, err = net.ListenUDP("udp", &net.UDPAddr{Port: int(m.Port)})
		if err != nil {
			return err
		}
	}
	m.udpConn = c
	log.Info().
		Str("label", m.Label).
		Str("transport", "udp").
		Str("local", c.LocalAddr().String()).
		Uint16("port", m.Port).
		Msg("[gb28181] media: listening")
	go m.readUDP()
	return nil
}

func (m *MediaReceiver) listenTCP() error {
	addr := &net.TCPAddr{IP: net.ParseIP(m.LocalIP), Port: int(m.Port)}
	if addr.IP == nil {
		addr.IP = net.IPv4zero
	}
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		ln, err = net.ListenTCP("tcp", &net.TCPAddr{Port: int(m.Port)})
		if err != nil {
			return err
		}
	}
	m.tcpLn = ln
	log.Info().
		Str("label", m.Label).
		Str("transport", "tcp-passive").
		Str("local", ln.Addr().String()).
		Uint16("port", m.Port).
		Msg("[gb28181] media: listening")
	go m.acceptTCP()
	return nil
}

func (m *MediaReceiver) noteRTP(remote string, pkt *rtp.Packet, rawLen int) {
	n := m.rtpCount.Add(1)
	m.rtpBytes.Add(uint64(len(pkt.Payload)))
	if !m.loggedFirst.Swap(true) {
		log.Info().
			Str("label", m.Label).
			Str("remote", remote).
			Int("raw", rawLen).
			Int("payload", len(pkt.Payload)).
			Uint8("pt", pkt.PayloadType).
			Uint16("seq", pkt.SequenceNumber).
			Uint32("ssrc", pkt.SSRC).
			Uint32("ts", pkt.Timestamp).
			Msg("[gb28181] media: first rtp")
	} else if n == 50 || n == 200 || n%500 == 0 {
		log.Info().
			Str("label", m.Label).
			Uint64("rtp", n).
			Uint64("bytes", m.rtpBytes.Load()).
			Uint64("bad", m.rtpBad.Load()).
			Str("remote", remote).
			Int("payload", len(pkt.Payload)).
			Msg("[gb28181] media: rtp stats")
	}
}

func (m *MediaReceiver) readUDP() {
	buf := make([]byte, 65535)
	pkt := &rtp.Packet{}
	for {
		n, remote, err := m.udpConn.ReadFromUDP(buf)
		if err != nil {
			log.Debug().Err(err).Str("label", m.Label).Msg("[gb28181] media: udp read end")
			return
		}
		if err := pkt.Unmarshal(buf[:n]); err != nil {
			bad := m.rtpBad.Add(1)
			if bad == 1 || bad%50 == 0 {
				log.Warn().
					Err(err).
					Str("label", m.Label).
					Str("remote", remote.String()).
					Int("raw", n).
					Uint64("bad", bad).
					Msg("[gb28181] media: rtp unmarshal failed")
			}
			continue
		}
		m.noteRTP(remote.String(), pkt, n)
		m.mu.Lock()
		fn := m.OnPayload
		m.mu.Unlock()
		if fn != nil {
			fn(pkt.Payload, pkt.Timestamp)
		}
	}
}

func (m *MediaReceiver) acceptTCP() {
	_ = m.tcpLn.SetDeadline(time.Now().Add(30 * time.Second))
	conn, err := m.tcpLn.Accept()
	if err != nil {
		log.Warn().Err(err).Str("label", m.Label).Msg("[gb28181] media: tcp accept failed")
		return
	}
	_ = m.tcpLn.SetDeadline(time.Time{})
	log.Info().
		Str("label", m.Label).
		Str("remote", conn.RemoteAddr().String()).
		Msg("[gb28181] media: tcp connected")
	m.mu.Lock()
	m.tcpConn = conn
	m.mu.Unlock()
	go m.readTCP(conn)
}

// TCP passive GB28181 often uses 2-byte length + RTP, or raw RTP.
func (m *MediaReceiver) readTCP(conn net.Conn) {
	defer conn.Close()
	hdr := make([]byte, 2)
	pkt := &rtp.Packet{}
	for {
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return
		}
		n := int(binary.BigEndian.Uint16(hdr))
		if n < 12 || n > 65535 {
			// maybe not length-prefixed — treat hdr as start of RTP
			buf := make([]byte, 2048)
			copy(buf, hdr)
			rn, err := conn.Read(buf[2:])
			if err != nil {
				return
			}
			if err := pkt.Unmarshal(buf[:2+rn]); err != nil {
				continue
			}
			m.emit(pkt)
			continue
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		if err := pkt.Unmarshal(buf); err != nil {
			continue
		}
		m.emit(pkt)
	}
}

func (m *MediaReceiver) emit(pkt *rtp.Packet) {
	remote := ""
	m.mu.Lock()
	if m.tcpConn != nil {
		remote = m.tcpConn.RemoteAddr().String()
	}
	fn := m.OnPayload
	m.mu.Unlock()
	m.noteRTP(remote, pkt, len(pkt.Payload)+12)
	if fn != nil {
		fn(pkt.Payload, pkt.Timestamp)
	}
}

func (m *MediaReceiver) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	var err error
	if m.udpConn != nil {
		err = m.udpConn.Close()
	}
	if m.tcpLn != nil {
		_ = m.tcpLn.Close()
	}
	if m.tcpConn != nil {
		_ = m.tcpConn.Close()
	}
	return err
}

func (m *MediaReceiver) LocalAddr() string {
	if m.udpConn != nil {
		return m.udpConn.LocalAddr().String()
	}
	if m.tcpLn != nil {
		return m.tcpLn.Addr().String()
	}
	return fmt.Sprintf(":%d", m.Port)
}
