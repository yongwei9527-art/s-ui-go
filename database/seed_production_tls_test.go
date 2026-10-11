package database

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	stdtls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/util"
	"gorm.io/gorm"
)

var productionEnvNames = []string{"S_UI_DEFAULT_TLS_SERVER_NAME", "S_UI_DEFAULT_TLS_CERTIFICATE_PATH", "S_UI_DEFAULT_TLS_KEY_PATH", "S_UI_DEFAULT_TLS_MIGRATE_BOOTSTRAP"}

func clearProductionEnv(t *testing.T) {
	t.Helper()
	for _, name := range productionEnvNames {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

// A local test CA issues this fixture. It is not presented as a public CA.
func productionCertificate(t *testing.T, host string, before, after time.Time) ([]byte, []byte) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Local test CA"}, IsCA: true, BasicConstraintsValid: true,
		NotBefore: time.Now().Add(-24 * time.Hour), NotAfter: time.Now().Add(48 * time.Hour), KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: before, NotAfter: after, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	cert = append(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})...)
	return cert, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func configureProductionTLS(t *testing.T, host string, cert, key []byte) {
	t.Helper()
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, cert, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(productionEnvNames[0], host)
	t.Setenv(productionEnvNames[1], certPath)
	t.Setenv(productionEnvNames[2], keyPath)
}

func productionDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.AutoMigrate(&model.Tls{}, &model.Inbound{}, &model.Client{}, &model.User{}, &model.XrayXHTTPInbound{}, &model.XrayXHTTPClient{}); err != nil {
		t.Fatal(err)
	}
	old := db
	db = database
	t.Cleanup(func() { db = old })
	return database
}

func decodeProductionJSON(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var value map[string]interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestDefaultProductionTLSValidation(t *testing.T) {
	now := time.Now()
	validCert, validKey := productionCertificate(t, "vpn.example.test", now.Add(-time.Hour), now.Add(time.Hour))
	_, otherKey := productionCertificate(t, "vpn.example.test", now.Add(-time.Hour), now.Add(time.Hour))
	expiredCert, expiredKey := productionCertificate(t, "vpn.example.test", now.Add(-2*time.Hour), now.Add(-time.Hour))
	futureCert, futureKey := productionCertificate(t, "vpn.example.test", now.Add(time.Hour), now.Add(2*time.Hour))
	for _, tc := range []struct {
		name, host string
		cert, key  []byte
		wantError  bool
	}{
		{"valid-chain", "vpn.example.test", validCert, validKey, false},
		{"wildcard", "vpn.example.test", nil, nil, false},
		{"hostname", "other.example.test", validCert, validKey, true},
		{"mismatched-key", "vpn.example.test", validCert, otherKey, true},
		{"malformed-cert", "vpn.example.test", []byte("not PEM"), validKey, true},
		{"malformed-key", "vpn.example.test", validCert, []byte("not PEM"), true},
		{"expired", "vpn.example.test", expiredCert, expiredKey, true},
		{"not-yet-valid", "vpn.example.test", futureCert, futureKey, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearProductionEnv(t)
			if tc.name == "wildcard" {
				tc.cert, tc.key = productionCertificate(t, "*.example.test", now.Add(-time.Hour), now.Add(time.Hour))
			}
			configureProductionTLS(t, tc.host, tc.cert, tc.key)
			config, err := loadDefaultProductionTLS()
			if (err != nil) != tc.wantError {
				t.Fatalf("config=%v err=%v", config, err)
			}
			if err == nil && (!bytes.Equal(config.certificate, tc.cert) || !bytes.Equal(config.key, tc.key)) {
				t.Fatal("supplied certificate/key changed")
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		clearProductionEnv(t)
		config, err := loadDefaultProductionTLS()
		if err != nil || config != nil {
			t.Fatalf("config=%v err=%v", config, err)
		}
	})
	for mask := 1; mask < 7; mask++ {
		t.Run(big.NewInt(int64(mask)).String(), func(t *testing.T) {
			clearProductionEnv(t)
			for i := 0; i < 3; i++ {
				if mask&(1<<i) != 0 {
					t.Setenv(productionEnvNames[i], "value")
				}
			}
			if _, err := loadDefaultProductionTLS(); err == nil {
				t.Fatal("partial config accepted")
			}
		})
	}
	for _, name := range productionEnvNames[:3] {
		t.Run("empty-"+name, func(t *testing.T) {
			clearProductionEnv(t)
			configureProductionTLS(t, "vpn.example.test", validCert, validKey)
			t.Setenv(name, "   ")
			if _, err := loadDefaultProductionTLS(); err == nil {
				t.Fatal("empty setting accepted")
			}
		})
	}
	t.Run("explicit-empty", func(t *testing.T) {
		clearProductionEnv(t)
		t.Setenv(productionEnvNames[0], "")
		if _, err := loadDefaultProductionTLS(); err == nil {
			t.Fatal("explicit empty setting accepted")
		}
	})
	for _, name := range productionEnvNames[1:3] {
		t.Run("missing-file-"+name, func(t *testing.T) {
			clearProductionEnv(t)
			configureProductionTLS(t, "vpn.example.test", validCert, validKey)
			t.Setenv(name, filepath.Join(t.TempDir(), "missing"))
			if _, err := loadDefaultProductionTLS(); err == nil {
				t.Fatal("missing file accepted")
			}
		})
	}
	t.Run("expired-chain-certificate", func(t *testing.T) {
		clearProductionEnv(t)
		// Append an expired certificate to a valid leaf chain.
		block, _ := pem.Decode(expiredCert)
		configureProductionTLS(t, "vpn.example.test", append(append([]byte{}, validCert...), pem.EncodeToMemory(block)...), validKey)
		if _, err := loadDefaultProductionTLS(); err == nil {
			t.Fatal("expired chain member accepted")
		}
	})
}

func TestDefaultProductionTLSFreshInstall(t *testing.T) {
	clearProductionEnv(t)
	cert, key := productionCertificate(t, "vpn.example.test", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	configureProductionTLS(t, "vpn.example.test", cert, key)
	database := productionDB(t)
	if err := seedDefaultTlsAndInbounds(); err != nil {
		t.Fatal(err)
	}
	var templates []model.Tls
	if err := database.Find(&templates).Error; err != nil {
		t.Fatal(err)
	}
	if len(templates) != 9 {
		t.Fatalf("templates=%d", len(templates))
	}
	for _, template := range templates {
		if template.Name == "reality-vless" {
			continue
		}
		if strings.HasSuffix(template.Name, selfSignedTLSBootstrapOnlySuffix) {
			t.Fatal("production is marked self-signed")
		}
		server := decodeProductionJSON(t, template.Server)
		client := decodeProductionJSON(t, template.Client)
		if server["server_name"] != "vpn.example.test" || client["server_name"] != "vpn.example.test" || client["insecure"] != false {
			t.Fatalf("bad TLS: %s", template.Client)
		}
		wantCert, _ := json.Marshal(pemLines(cert))
		gotCert, _ := json.Marshal(server["certificate"])
		wantKey, _ := json.Marshal(pemLines(key))
		gotKey, _ := json.Marshal(server["key"])
		if !bytes.Equal(wantCert, gotCert) || !bytes.Equal(wantKey, gotKey) {
			t.Fatal("supplied PEM replaced")
		}
	}
	var inbounds []model.Inbound
	if err := database.Preload("Tls").Find(&inbounds).Error; err != nil {
		t.Fatal(err)
	}
	for _, inbound := range inbounds {
		if inbound.Tls == nil || inbound.Tls.Name == "reality-vless" {
			continue
		}
		tls := decodeProductionJSON(t, inbound.OutJson)["tls"].(map[string]interface{})
		if tls["server_name"] != "vpn.example.test" || tls["insecure"] != false {
			t.Fatalf("bad outbound: %s", inbound.OutJson)
		}
		if strings.Contains(string(inbound.Addrs), "\"insecure\":true") {
			t.Fatal("stale address insecure override")
		}
	}
	var before []model.Tls
	_ = database.Order("id").Find(&before).Error
	if err := seedDefaultTlsAndInbounds(); err != nil {
		t.Fatal(err)
	}
	var after []model.Tls
	_ = database.Order("id").Find(&after).Error
	if !reflect.DeepEqual(before, after) {
		t.Fatal("restart overwrote production TLS")
	}
}

func TestDefaultProductionTLSInvalidConfigDoesNotSeed(t *testing.T) {
	clearProductionEnv(t)
	t.Setenv(productionEnvNames[0], "vpn.example.test")
	database := productionDB(t)
	if err := seedDefaultTlsAndInbounds(); err == nil {
		t.Fatal("partial config accepted")
	}
	for _, model := range []interface{}{&model.Tls{}, &model.Inbound{}, &model.Client{}} {
		var count int64
		if err := database.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("count=%d err=%v", count, err)
		}
	}
}

func TestDefaultProductionTLSMigrationExportsAndPreservesData(t *testing.T) {
	clearProductionEnv(t)
	database := productionDB(t)
	seeded, err := seedDefaultTlsTemplates(database)
	if err != nil {
		t.Fatal(err)
	}
	inbounds, err := seedDefaultInbounds(database, seeded, "192.0.2.10")
	if err != nil {
		t.Fatal(err)
	}
	if err := seedDefaultClient(database, inbounds, "192.0.2.10"); err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "operator", Password: "keep-hash"}
	if err := database.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	// Include an unmarked legacy name and nested address TLS override.
	var legacy model.Tls
	if err := database.First(&legacy, seeded[bootstrapOnlySelfSignedTLSName("tls-hy2")].id).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&legacy).Update("name", "tls-hy2").Error; err != nil {
		t.Fatal(err)
	}
	var hy2 model.Inbound
	if err := database.Where("type = ?", "hysteria2").First(&hy2).Error; err != nil {
		t.Fatal(err)
	}
	addr := []map[string]interface{}{{"server": "192.0.2.10", "server_port": 8443, "remark": "keep", "insecure": true, "server_name": defaultBootstrapTLSName, "tls": map[string]interface{}{"enabled": true, "insecure": true, "server_name": defaultBootstrapTLSName}}}
	hy2.Addrs, _ = json.Marshal(addr)
	if err := database.Model(&hy2).Update("addrs", hy2.Addrs).Error; err != nil {
		t.Fatal(err)
	}
	var beforeInbounds []model.Inbound
	_ = database.Order("id").Find(&beforeInbounds).Error
	var beforeClients []model.Client
	_ = database.Order("id").Find(&beforeClients).Error
	var beforeTLS []model.Tls
	_ = database.Order("id").Find(&beforeTLS).Error
	cert, key := productionCertificate(t, "vpn.example.test", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	configureProductionTLS(t, "vpn.example.test", cert, key)
	// Providing the three settings alone does not migrate.
	if err := seedDefaultTlsAndInbounds(); err != nil {
		t.Fatal(err)
	}
	var noOptIn []model.Tls
	_ = database.Order("id").Find(&noOptIn).Error
	if !reflect.DeepEqual(beforeTLS, noOptIn) {
		t.Fatal("migrated without opt-in")
	}
	t.Setenv(productionEnvNames[3], "true")
	if err := seedDefaultTlsAndInbounds(); err != nil {
		t.Fatal(err)
	}
	var migrated []model.Inbound
	if err := database.Preload("Tls").Order("id").Find(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	for i, inbound := range migrated {
		before := beforeInbounds[i]
		if before.Id != inbound.Id || before.Tag != inbound.Tag || before.Type != inbound.Type || before.TlsId != inbound.TlsId || !bytes.Equal(before.Options, inbound.Options) {
			t.Fatal("listener/id changed")
		}
		if inbound.Tls == nil || inbound.Tls.Name == "reality-vless" {
			if !bytes.Equal(before.OutJson, inbound.OutJson) {
				t.Fatal("unrelated outbound changed")
			}
			continue
		}
		export := decodeProductionJSON(t, inbound.OutJson)
		tls := export["tls"].(map[string]interface{})
		if tls["server_name"] != "vpn.example.test" || tls["insecure"] != false || export["server"] != "192.0.2.10" {
			t.Fatalf("stale export: %s", inbound.OutJson)
		}
		// MarshalFull is the server-side config export path.
		_, err := inbound.MarshalFull()
		if err != nil {
			t.Fatal(err)
		}
		if decodeProductionJSON(t, inbound.Tls.Server)["server_name"] != "vpn.example.test" {
			t.Fatal("server export has stale SNI")
		}
		if inbound.Type == "hysteria2" {
			links := util.LinkGenerator(json.RawMessage(`{"hysteria2":{"password":"unchanged"}}`), &inbound, "192.0.2.10")
			if len(links) != 1 {
				t.Fatalf("links=%v", links)
			}
			link, err := url.Parse(links[0])
			if err != nil {
				t.Fatal(err)
			}
			if link.Query().Get("sni") != "vpn.example.test" || link.Query().Get("insecure") == "1" || link.Query().Get("allowInsecure") == "1" {
				t.Fatalf("stale link %s", links[0])
			}
		}
	}
	var afterClients []model.Client
	_ = database.Order("id").Find(&afterClients).Error
	if !reflect.DeepEqual(beforeClients, afterClients) {
		t.Fatal("client credentials/assignments changed")
	}
	var afterUser model.User
	_ = database.First(&afterUser, user.Id).Error
	if !reflect.DeepEqual(user, afterUser) {
		t.Fatal("user changed")
	}
	var firstTLS []model.Tls
	var firstInbounds []model.Inbound
	_ = database.Order("id").Find(&firstTLS).Error
	_ = database.Order("id").Find(&firstInbounds).Error
	if err := seedDefaultTlsAndInbounds(); err != nil {
		t.Fatal(err)
	}
	var secondTLS []model.Tls
	var secondInbounds []model.Inbound
	_ = database.Order("id").Find(&secondTLS).Error
	_ = database.Order("id").Find(&secondInbounds).Error
	if !reflect.DeepEqual(firstTLS, secondTLS) || !reflect.DeepEqual(firstInbounds, secondInbounds) {
		t.Fatal("migration not idempotent")
	}
}

func TestDefaultProductionTLSMigrationStrictMatch(t *testing.T) {
	clearProductionEnv(t)
	database := productionDB(t)
	spec := legacyBootstrapOnlyTLSSpecs[0]
	base, err := buildBootstrapOnlySelfSignedTLS(spec.name, spec.serverName, spec.alpn)
	if err != nil {
		t.Fatal(err)
	}
	prodCert, prodKey := productionCertificate(t, spec.serverName, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	for _, change := range []string{"name", "sni", "alpn", "insecure", "extra-server", "extra-client", "extra-utls", "malformed-pem", "key-mismatch", "ca-issued"} {
		t.Run(change, func(t *testing.T) {
			candidate := base
			server := decodeProductionJSON(t, base.Server)
			client := decodeProductionJSON(t, base.Client)
			switch change {
			case "name":
				candidate.Name = "custom"
			case "sni":
				server["server_name"] = "custom.test"
			case "alpn":
				client["alpn"] = []string{"h2"}
			case "insecure":
				client["insecure"] = false
			case "extra-server":
				server["certificate_path"] = "custom.pem"
			case "extra-client":
				client["disable_sni"] = false
			case "extra-utls":
				client["utls"].(map[string]interface{})["extra"] = true
			case "malformed-pem":
				server["certificate"] = []string{"not PEM"}
			case "key-mismatch":
				server["key"] = pemLines(prodKey)
			case "ca-issued":
				server["certificate"], server["key"] = pemLines(prodCert), pemLines(prodKey)
			}
			candidate.Server, _ = json.Marshal(server)
			candidate.Client, _ = json.Marshal(client)
			if matchesUnmodifiedBootstrapTLS(candidate, spec) {
				t.Fatal("custom template matched")
			}
			if err := database.Create(&candidate).Error; err != nil {
				t.Fatal(err)
			}
			config := &defaultProductionTLS{serverName: "vpn.example.test", certificate: prodCert, key: prodKey}
			if err := migrateDefaultProductionTLS(database, config); err != nil {
				t.Fatal(err)
			}
			var after model.Tls
			if err := database.First(&after, candidate.Id).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(candidate, after) {
				t.Fatal("custom TLS overwritten")
			}
		})
	}
	if !matchesUnmodifiedBootstrapTLS(base, spec) {
		t.Fatal("generated bootstrap did not match")
	}
	pair, err := stdtls.X509KeyPair(prodCert, prodKey)
	if err != nil || len(pair.Certificate) != 2 {
		t.Fatal("invalid fixture chain")
	}
}

func TestDefaultProductionTLSMigrationRollback(t *testing.T) {
	clearProductionEnv(t)
	database := productionDB(t)
	seeded, err := seedDefaultTlsTemplates(database)
	if err != nil {
		t.Fatal(err)
	}
	var before []model.Tls
	_ = database.Order("id").Find(&before).Error
	bad := model.Inbound{Type: "hysteria2", Tag: "bad", TlsId: seeded[bootstrapOnlySelfSignedTLSName("tls-hy2")].id, Options: json.RawMessage(`{"listen_port":8443}`), OutJson: json.RawMessage(`{invalid`)}
	if err := database.Create(&bad).Error; err != nil {
		t.Fatal(err)
	}
	cert, key := productionCertificate(t, "vpn.example.test", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	configureProductionTLS(t, "vpn.example.test", cert, key)
	t.Setenv(productionEnvNames[3], "true")
	if err := seedDefaultTlsAndInbounds(); err == nil {
		t.Fatal("bad OutJson accepted")
	}
	var after []model.Tls
	_ = database.Order("id").Find(&after).Error
	if !reflect.DeepEqual(before, after) {
		t.Fatal("TLS changes were not rolled back")
	}
}
