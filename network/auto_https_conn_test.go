package network

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// scriptedConn preserves TCP read boundaries without timing-dependent sleeps.
type scriptedConn struct {
	chunks    [][]byte
	written   bytes.Buffer
	closed    bool
	readBytes int
}

func (c *scriptedConn) Read(p []byte) (int, error) {
	if c.closed {
		return 0, net.ErrClosed
	}
	if len(c.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.chunks[0])
	c.readBytes += n
	c.chunks[0] = c.chunks[0][n:]
	if len(c.chunks[0]) == 0 {
		c.chunks = c.chunks[1:]
	}
	return n, nil
}
func (c *scriptedConn) Write(p []byte) (int, error)    { return c.written.Write(p) }
func (c *scriptedConn) Close() error                   { c.closed = true; return nil }
func (*scriptedConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (*scriptedConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (*scriptedConn) SetDeadline(time.Time) error      { return nil }
func (*scriptedConn) SetReadDeadline(time.Time) error  { return nil }
func (*scriptedConn) SetWriteDeadline(time.Time) error { return nil }

func TestAutoHTTPSRedirectsFragmentedAndLargeHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		chunks [][]byte
	}{
		{"fragmented", [][]byte{[]byte("G"), []byte("ET /sub?id=1 HTTP/1.1\r\n"), []byte("Host: example.test:8443\r\n\r\n")}},
		{"large", [][]byte{[]byte("GET /sub?id=1 HTTP/1.1\r\nHost: example.test:8443\r\nX-Padding: " + strings.Repeat("a", 4096) + "\r\n\r\n")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := &scriptedConn{chunks: tc.chunks}
			_, _ = NewAutoHttpsConn(raw).Read(make([]byte, 512))
			resp, err := http.ReadResponse(bufio.NewReader(&raw.written), nil)
			if err != nil {
				t.Fatalf("missing HTTP redirect: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusTemporaryRedirect || resp.Header.Get("Location") != "https://example.test:8443/sub?id=1" {
				t.Fatalf("unexpected redirect: %v", resp)
			}
			if !raw.closed {
				t.Fatal("redirected connection not closed")
			}
		})
	}
}

func TestAutoHTTPSTLSPassthrough(t *testing.T) {
	data := []byte{0x16, 0x03, 0x03, 0x00, 0x04, 1, 2, 3, 4}
	raw := &scriptedConn{chunks: [][]byte{data[:1], data[1:]}}
	conn := NewAutoHttpsConn(raw)
	first := make([]byte, 1)
	if n, err := conn.Read(first); n != 1 || err != nil || raw.readBytes != 1 {
		t.Fatalf("TLS sniff read ahead: n=%d, err=%v, consumed=%d", n, err, raw.readBytes)
	}
	rest, err := io.ReadAll(conn)
	got := append(first, rest...)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("TLS bytes changed: %x, %v", got, err)
	}
	if raw.closed || raw.written.Len() != 0 {
		t.Fatal("TLS connection was redirected")
	}
}

func TestAutoHTTPSFragmentedHeaderHonorsReadDeadline(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	if err := server.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = client.Write([]byte("GET / HTTP/1.1\r\n")) }()
	conn := NewAutoHttpsConn(server)
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil || string(buf[:n]) != "GET / HTTP/1.1\r\n" {
		t.Fatalf("incomplete request not preserved: %q, %v", buf[:n], err)
	}
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("underlying read deadline was ignored")
	}
}

func TestAutoHTTPSHeaderSniffIsBounded(t *testing.T) {
	const limit = 16 << 10
	data := []byte("GET / HTTP/1.1\r\nHost: example.test\r\nX-Padding: " + strings.Repeat("a", 2*limit) + "\r\n\r\n")
	raw := &scriptedConn{chunks: [][]byte{data}}
	conn := NewAutoHttpsConn(raw)
	buf := make([]byte, 1)
	if n, err := conn.Read(buf); n != 1 || err != nil {
		t.Fatalf("Read = %d, %v", n, err)
	}
	if raw.readBytes > limit {
		t.Fatalf("sniff consumed %d bytes, limit %d", raw.readBytes, limit)
	}
	rest, err := io.ReadAll(conn)
	if err != nil || !bytes.Equal(append(buf, rest...), data) {
		t.Fatal("oversized input was not preserved")
	}
	if raw.closed || raw.written.Len() != 0 {
		t.Fatal("oversized header was redirected")
	}
}
