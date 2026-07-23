package app

import "testing"

func TestProxyCoresRequired(t *testing.T) {
	for _, value := range []string{"1", " true ", "YES"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SUI_REQUIRE_PROXY_CORES", value)
			if !proxyCoresRequired() {
				t.Fatalf("SUI_REQUIRE_PROXY_CORES=%q was not enabled", value)
			}
		})
	}
	for _, value := range []string{"", "0", "false", "no", "unexpected"} {
		t.Run("disabled-"+value, func(t *testing.T) {
			t.Setenv("SUI_REQUIRE_PROXY_CORES", value)
			if proxyCoresRequired() {
				t.Fatalf("SUI_REQUIRE_PROXY_CORES=%q was unexpectedly enabled", value)
			}
		})
	}
}
