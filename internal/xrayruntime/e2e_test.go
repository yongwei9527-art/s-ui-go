package xrayruntime

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	xproxy "golang.org/x/net/proxy"

	// Client-only registrations used by this end-to-end test. They are linked
	// into the test binary, not the production S-UI executable.
	_ "github.com/xtls/xray-core/proxy/socks"
	_ "github.com/xtls/xray-core/proxy/vless/outbound"
)

func TestRealityTransportsEndToEnd(t *testing.T) {
	const publicKey = "E59WjnvZcQMu7tR7_BgyhycuEdBS-CtKxfImRCdAvFM"
	realityTarget := startLocalRealityTarget(t)
	echoTarget := startLocalTCPEcho(t)

	for _, transport := range []string{"tcp", "xhttp", "xhttp-stream-one"} {
		t.Run(transport, func(t *testing.T) {
			serverPort := unusedLoopbackPort(t)

			server, err := New(e2eServerConfig(transport, serverPort, realityTarget))
			if err != nil {
				t.Fatalf("prepare server: %v", err)
			}
			closed := false
			t.Cleanup(func() {
				if !closed {
					_ = server.Close()
				}
			})
			if err := server.Start(); err != nil {
				t.Fatalf("start server: %v", err)
			}

			runE2EClientProcess(t, transport, serverPort, echoTarget, publicKey)
			if err := server.Close(); err != nil {
				t.Fatalf("close server: %v", err)
			}
			closed = true
		})
	}
}

func TestRealityTransportClientProcess(t *testing.T) {
	if os.Getenv("SUI_XRAY_E2E_CLIENT") != "1" {
		t.Skip("helper process")
	}
	transport := os.Getenv("SUI_XRAY_E2E_TRANSPORT")
	serverPort, err := strconv.Atoi(os.Getenv("SUI_XRAY_E2E_SERVER_PORT"))
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}
	echoTarget := os.Getenv("SUI_XRAY_E2E_ECHO_TARGET")
	publicKey := os.Getenv("SUI_XRAY_E2E_PUBLIC_KEY")
	socksPort := unusedLoopbackPort(t)
	for socksPort == serverPort {
		socksPort = unusedLoopbackPort(t)
	}

	client, err := New(e2eClientConfig(transport, serverPort, socksPort, publicKey))
	if err != nil {
		t.Fatalf("prepare client: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := client.Close(); err != nil {
				t.Errorf("cleanup client: %v", err)
			}
		}
	})
	if err := client.Start(); err != nil {
		t.Fatalf("start client: %v", err)
	}
	payload := []byte("s-ui-xray-" + transport + "-round-trip")
	reply := roundTripThroughSOCKS(t, socksPort, echoTarget, payload)
	if !bytes.Equal(reply, payload) {
		t.Fatalf("round-trip reply = %q, want %q", reply, payload)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	closed = true
}

func runE2EClientProcess(t *testing.T, transport string, serverPort int, echoTarget, publicKey string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0],
		"-test.run=^TestRealityTransportClientProcess$",
		"-test.v",
		"-test.timeout=25s",
	)
	command.Env = append(os.Environ(),
		"SUI_XRAY_E2E_CLIENT=1",
		"SUI_XRAY_E2E_TRANSPORT="+transport,
		"SUI_XRAY_E2E_SERVER_PORT="+strconv.Itoa(serverPort),
		"SUI_XRAY_E2E_ECHO_TARGET="+echoTarget,
		"SUI_XRAY_E2E_PUBLIC_KEY="+publicKey,
	)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("client process timed out: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("client process failed: %v\n%s", err, output)
	}
}

func startLocalRealityTarget(t *testing.T) string {
	t.Helper()
	certificate := newLocalTLSCertificate(t)
	listener, err := tls.Listen("tcp4", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h2", "http/1.1"},
	})
	if err != nil {
		t.Fatalf("start local Reality target: %v", err)
	}
	tracked := newTrackedTestListener(listener, func(conn net.Conn) {
		defer conn.Close()
		// Xray's RAW/TCP Reality listener probes the camouflage target for
		// post-handshake TLS records with a five-second read window. Keep the
		// real target connection alive beyond that probe window so the first
		// authenticated Vision connection cannot lose a deadline race.
		_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
		buffer := make([]byte, 4096)
		_, _ = conn.Read(buffer)
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	})
	t.Cleanup(func() { _ = tracked.Close() })
	return listener.Addr().String()
}

func newLocalTLSCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate target key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate target serial: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "www.example.com"},
		DNSNames:     []string{"www.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create target certificate: %v", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal target key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatalf("parse target certificate: %v", err)
	}
	return certificate
}

func startLocalTCPEcho(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start echo target: %v", err)
	}
	tracked := newTrackedTestListener(listener, func(conn net.Conn) {
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	})
	t.Cleanup(func() { _ = tracked.Close() })
	return listener.Addr().String()
}

type trackedTestListener struct {
	net.Listener
	mu         sync.Mutex
	active     map[net.Conn]struct{}
	handlers   sync.WaitGroup
	acceptDone chan struct{}
}

func newTrackedTestListener(listener net.Listener, handle func(net.Conn)) *trackedTestListener {
	tracked := &trackedTestListener{
		Listener:   listener,
		active:     make(map[net.Conn]struct{}),
		acceptDone: make(chan struct{}),
	}
	go func() {
		defer close(tracked.acceptDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			tracked.mu.Lock()
			tracked.active[conn] = struct{}{}
			tracked.handlers.Add(1)
			tracked.mu.Unlock()
			go func() {
				defer tracked.handlers.Done()
				defer func() {
					tracked.mu.Lock()
					delete(tracked.active, conn)
					tracked.mu.Unlock()
				}()
				handle(conn)
			}()
		}
	}()
	return tracked
}

func (l *trackedTestListener) Close() error {
	err := l.Listener.Close()
	<-l.acceptDone
	l.mu.Lock()
	for conn := range l.active {
		_ = conn.Close()
	}
	l.mu.Unlock()
	l.handlers.Wait()
	return err
}

func roundTripThroughSOCKS(t *testing.T, socksPort int, target string, payload []byte) []byte {
	t.Helper()
	dialer, err := xproxy.SOCKS5("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(socksPort)), nil, xproxy.Direct)
	if err != nil {
		t.Fatalf("create SOCKS dialer: %v", err)
	}
	var conn net.Conn
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err = dialer.Dial("tcp", target)
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial %s through SOCKS: %v", target, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write proxied payload: %v", err)
	}
	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read proxied payload: %v", err)
	}
	return reply
}

func e2eServerConfig(transport string, port int, realityTarget string) []byte {
	path, mode := "/xhttp", "auto"
	if transport == "xhttp-stream-one" {
		transport, path, mode = "xhttp", "/xhttp-stream", "stream-one"
	}
	transportSettings := ""
	clientFlow := ""
	if transport == "xhttp" {
		transportSettings = fmt.Sprintf(`,"xhttpSettings":{"path":%q,"mode":%q}`, path, mode)
	} else {
		clientFlow = `,"flow":"xtls-rprx-vision"`
	}
	return []byte(fmt.Sprintf(`{
		"log":{"loglevel":"none"},
		"inbounds":[{
			"tag":"reality-%s-server",
			"listen":"127.0.0.1",
			"port":%d,
			"protocol":"vless",
			"settings":{"decryption":"none","clients":[{"id":%q%s}]},
			"streamSettings":{
				"network":%q,
				"security":"reality",
				"realitySettings":{
					"show":false,
					"dest":%q,
					"xver":0,
					"serverNames":["www.example.com"],
					"privateKey":%q,
					"shortIds":["0123456789abcdef"]
				}%s
			}
		}],
		"outbounds":[
			{"protocol":"freedom","tag":"direct"},
			{"protocol":"blackhole","tag":"blocked"}
		]
	}`, transport, port, testUUID, clientFlow, transport, realityTarget, testRealityPrivateKey, transportSettings))
}

func e2eClientConfig(transport string, serverPort, socksPort int, publicKey string) []byte {
	path, mode := "/xhttp", "auto"
	if transport == "xhttp-stream-one" {
		transport, path, mode = "xhttp", "/xhttp-stream", "stream-one"
	}
	transportSettings := ""
	clientFlow := ""
	if transport == "xhttp" {
		transportSettings = fmt.Sprintf(`,"xhttpSettings":{"path":%q,"mode":%q}`, path, mode)
	} else {
		clientFlow = `,"flow":"xtls-rprx-vision"`
	}
	return []byte(fmt.Sprintf(`{
		"log":{"loglevel":"none"},
		"inbounds":[{
			"tag":"local-socks",
			"listen":"127.0.0.1",
			"port":%d,
			"protocol":"socks",
			"settings":{"auth":"noauth","udp":false}
		}],
		"outbounds":[{
			"tag":"xray-server",
			"protocol":"vless",
			"settings":{"vnext":[{
				"address":"127.0.0.1",
				"port":%d,
				"users":[{"id":%q,"encryption":"none"%s}]
			}]},
			"streamSettings":{
				"network":%q,
				"security":"reality",
				"realitySettings":{
					"serverName":"www.example.com",
					"fingerprint":"chrome",
					"publicKey":%q,
					"shortId":"0123456789abcdef",
					"spiderX":"/"
				}%s
			}
		}]
	}`, socksPort, serverPort, testUUID, clientFlow, transport, publicKey, transportSettings))
}
