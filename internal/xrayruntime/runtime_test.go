package xrayruntime

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

const (
	testRealityPrivateKey = "aGSYystUbf59_9_6LKRxD27rmSW_-2_nyd9YG_Gwbks"
	testUUID              = "11111111-1111-1111-1111-111111111111"
)

func TestNewAcceptsRequiredRealityConfigs(t *testing.T) {
	server, err := New(testConfig(12001, 12002))
	if err != nil {
		t.Fatalf("New rejected VLESS Reality TCP Vision/XHTTP config: %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("close prepared server: %v", err)
	}
}

func TestNewRejectsInvalidJSON(t *testing.T) {
	if _, err := New([]byte(`{"inbounds":[`)); err == nil {
		t.Fatal("New accepted invalid JSON")
	}
	if err := Validate([]byte(`{"inbounds":[`)); err == nil {
		t.Fatal("Validate accepted invalid JSON")
	}
}

func TestValidateDoesNotConstructASecondProcessInstance(t *testing.T) {
	port := unusedLoopbackPort(t)
	server, err := New(testConfig(port, 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if err := server.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := Validate(testConfig(port, 0)); err != nil {
		t.Fatalf("Validate while the current instance is running: %v", err)
	}
	if _, err := New(testConfig(unusedLoopbackPort(t), 0)); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second process instance error = %v, want already exists", err)
	}
}

func TestServerStartsBothTransportsOnLoopback(t *testing.T) {
	tcpPort := unusedLoopbackPort(t)
	xhttpPort := unusedLoopbackPort(t)
	for xhttpPort == tcpPort {
		xhttpPort = unusedLoopbackPort(t)
	}

	server, err := New(testConfig(tcpPort, xhttpPort))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	if err := server.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("second Start must be idempotent: %v", err)
	}
	waitForTCP(t, tcpPort, true)
	waitForTCP(t, xhttpPort, true)

	if err := server.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("second Close must be idempotent: %v", err)
	}
	waitForTCP(t, tcpPort, false)
	waitForTCP(t, xhttpPort, false)
	if err := server.Start(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Start after Close error = %v, want closed error", err)
	}
}

func TestFailedStartRetiresInstance(t *testing.T) {
	blocker, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback port: %v", err)
	}
	port := blocker.Addr().(*net.TCPAddr).Port

	server, err := New(testConfig(port, 0))
	if err != nil {
		_ = blocker.Close()
		t.Fatalf("New: %v", err)
	}
	if err := server.Start(); err == nil {
		_ = blocker.Close()
		_ = server.Close()
		t.Fatal("Start unexpectedly succeeded on an occupied port")
	}
	if err := blocker.Close(); err != nil {
		t.Fatalf("release occupied port: %v", err)
	}

	// Xray states that an instance must not be started again after any Start
	// attempt. The wrapper must retire a partially started instance.
	if err := server.Start(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("second Start error = %v, want retired/closed instance", err)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("Close after failed Start: %v", err)
	}
}

func unusedLoopbackPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate loopback port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release loopback port: %v", err)
	}
	return port
}

func waitForTCP(t *testing.T, port int, wantOpen bool) {
	t.Helper()
	address := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp4", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
		}
		if (err == nil) == wantOpen {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("listener %s open=%t, want %t (last error: %v)", address, err == nil, wantOpen, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func testConfig(tcpPort, xhttpPort int) []byte {
	inbounds := make([]string, 0, 2)
	if tcpPort > 0 {
		inbounds = append(inbounds, fmt.Sprintf(`{
			"tag":"reality-tcp",
			"listen":"127.0.0.1",
			"port":%d,
			"protocol":"vless",
			"settings":{"decryption":"none","clients":[{"id":%q,"flow":"xtls-rprx-vision"}]},
			"streamSettings":{
				"network":"tcp",
				"security":"reality",
				"realitySettings":{
					"show":false,
					"dest":"www.example.com:443",
					"xver":0,
					"serverNames":["www.example.com"],
					"privateKey":%q,
					"shortIds":["0123456789abcdef"]
				}
			}
		}`, tcpPort, testUUID, testRealityPrivateKey))
	}
	if xhttpPort > 0 {
		inbounds = append(inbounds, fmt.Sprintf(`{
			"tag":"reality-xhttp",
			"listen":"127.0.0.1",
			"port":%d,
			"protocol":"vless",
			"settings":{"decryption":"none","clients":[{"id":%q}]},
			"streamSettings":{
				"network":"xhttp",
				"security":"reality",
				"realitySettings":{
					"show":false,
					"dest":"www.example.com:443",
					"xver":0,
					"serverNames":["www.example.com"],
					"privateKey":%q,
					"shortIds":["0123456789abcdef"]
				},
				"xhttpSettings":{"path":"/xhttp","mode":"auto"}
			}
		}`, xhttpPort, testUUID, testRealityPrivateKey))
	}
	return []byte(fmt.Sprintf(`{
		"log":{"loglevel":"none"},
		"inbounds":[%s],
		"outbounds":[
			{"protocol":"freedom","tag":"direct"},
			{"protocol":"blackhole","tag":"blocked"}
		]
	}`, strings.Join(inbounds, ",")))
}
