package database

import (
	"encoding/json"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"testing"
)

func TestDefaultBootstrapTLSUsesReservedIdentityAndUniqueKeys(t *testing.T) {
	clearProductionEnv(t)
	db := productionDB(t)
	if _, err := seedDefaultTlsTemplates(db); err != nil {
		t.Fatal(err)
	}
	var templates []model.Tls
	if err := db.Find(&templates).Error; err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	count := 0
	for _, tpl := range templates {
		if tpl.Name == "reality-vless" {
			continue
		}
		var server, client map[string]interface{}
		if err := json.Unmarshal(tpl.Server, &server); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(tpl.Client, &client); err != nil {
			t.Fatal(err)
		}
		if server["server_name"] != defaultBootstrapTLSName || client["server_name"] != defaultBootstrapTLSName || client["insecure"] != true {
			t.Fatalf("invalid bootstrap settings: %s", tpl.Name)
		}
		raw, err := json.Marshal(server["key"])
		if err != nil {
			t.Fatal(err)
		}
		if keys[string(raw)] {
			t.Fatal("bootstrap nodes reused a private key")
		}
		keys[string(raw)] = true
		count++
	}
	if count != 8 {
		t.Fatalf("bootstrap templates=%d want 8", count)
	}
}
