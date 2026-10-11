package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
)

func TestVMessAllowInsecureRespectsValue(t *testing.T) {
	for _, tc := range []struct {
		value interface{}
		want  bool
	}{
		{false, false}, {"false", false}, {"0", false}, {0, false}, {nil, false}, {"", false}, {"invalid", false},
		{true, true}, {"true", true}, {"1", true}, {1, true},
	} {
		t.Run(fmt.Sprintf("%T-%v", tc.value, tc.value), func(t *testing.T) {
			data := map[string]interface{}{"add": "vpn.example.test", "port": "443", "id": "11111111-1111-1111-1111-111111111111", "net": "ws", "path": "/vmess", "tls": "tls", "sni": "vpn.example.test", "allowInsecure": tc.value}
			raw, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			out, _, err := GetOutbound("vmess://"+base64.StdEncoding.EncodeToString(raw), 0)
			if err != nil {
				t.Fatal(err)
			}
			tls, ok := (*out)["tls"].(map[string]interface{})
			if !ok {
				t.Fatalf("missing TLS: %v", out)
			}
			if (tls["insecure"] == true) != tc.want {
				t.Fatalf("insecure=%v want %v", tls["insecure"], tc.want)
			}
		})
	}
}
