//go:build with_utls

package xrayruntime

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/op/go-logging"
	suicore "github.com/yongwei9527-art/s-ui-go/core"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

func TestSingBoxAndEmbeddedXrayOperateTogether(t *testing.T) {
	const publicKey = "E59WjnvZcQMu7tR7_BgyhycuEdBS-CtKxfImRCdAvFM"
	logger.InitLogger(logging.ERROR)
	realityTarget := startLocalRealityTarget(t)
	echoTarget := startLocalTCPEcho(t)
	singPort := unusedLoopbackPort(t)
	xrayPort := unusedLoopbackPort(t)
	for xrayPort == singPort {
		xrayPort = unusedLoopbackPort(t)
	}

	singCore := suicore.NewCore()
	if err := singCore.Start(e2eSingBoxServerConfig(singPort, realityTarget)); err != nil {
		t.Fatalf("start sing-box Reality server: %v", err)
	}
	t.Cleanup(func() { _ = singCore.Stop() })

	xrayServer, err := New(e2eServerConfig("xhttp", xrayPort, realityTarget))
	if err != nil {
		t.Fatalf("prepare embedded Xray server: %v", err)
	}
	if err := xrayServer.Start(); err != nil {
		_ = xrayServer.Close()
		t.Fatalf("start embedded Xray server: %v", err)
	}
	t.Cleanup(func() { _ = xrayServer.Close() })

	// Use real Xray clients in isolated subprocesses. The TCP Vision request
	// proves that a standard Xray/v2rayNG-style client can use the sing-box
	// Reality default while the XHTTP request exercises the embedded server.
	runE2EClientProcess(t, "tcp", singPort, echoTarget, publicKey)
	runE2EClientProcess(t, "xhttp", xrayPort, echoTarget, publicKey)
}

func e2eSingBoxServerConfig(port int, realityTarget string) []byte {
	targetHost, targetPortText, err := net.SplitHostPort(realityTarget)
	if err != nil {
		panic(fmt.Sprintf("invalid local Reality target %q: %v", realityTarget, err))
	}
	targetPort, err := strconv.Atoi(targetPortText)
	if err != nil {
		panic(fmt.Sprintf("invalid local Reality target port %q: %v", targetPortText, err))
	}
	return []byte(fmt.Sprintf(`{
		"log":{"level":"error"},
		"inbounds":[{
			"type":"vless",
			"tag":"sing-reality-server",
			"listen":"127.0.0.1",
			"listen_port":%d,
			"users":[{"uuid":%q,"flow":"xtls-rprx-vision"}],
			"tls":{
				"enabled":true,
				"server_name":"www.example.com",
				"reality":{
					"enabled":true,
					"handshake":{"server":%q,"server_port":%d},
					"private_key":%q,
					"short_id":["0123456789abcdef"]
				}
			}
		}],
		"outbounds":[{"type":"direct","tag":"direct"}],
		"route":{"final":"direct"}
	}`, port, testUUID, targetHost, targetPort, testRealityPrivateKey))
}
