package app

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

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

func TestStartReturnsXrayFailureRegardlessOfProxyCoreRequirement(t *testing.T) {
	xrayErr := errors.New("xray unavailable")
	for _, requireProxyCores := range []string{"0", "1"} {
		t.Run("SUI_REQUIRE_PROXY_CORES="+requireProxyCores, func(t *testing.T) {
			t.Setenv("SUI_REQUIRE_PROXY_CORES", requireProxyCores)
			app, _ := newLifecycleTestApp(func(name string) error {
				if name == "start xray" {
					return xrayErr
				}
				return nil
			})

			err := app.Start()
			if !errors.Is(err, xrayErr) {
				t.Fatalf("Start() error = %v, want wrapped %v", err, xrayErr)
			}
		})
	}
}

func TestStartSkipsXrayWithoutEnabledInbound(t *testing.T) {
	t.Setenv("SUI_REQUIRE_PROXY_CORES", "0")
	app, events := newLifecycleTestApp(func(name string) error {
		if name == "start xray" {
			return errors.New("Xray must not start without an enabled inbound")
		}
		return nil
	})
	app.lifecycle.hasEnabledXrayInbound = func() (bool, error) { return false, nil }

	if err := app.Start(); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	want := []string{"start cron", "start web", "start sub", "start core"}
	if got := *events; !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle events = %v, want %v", got, want)
	}
}

func TestStartCleansStartedComponentsAfterXrayFailure(t *testing.T) {
	xrayErr := errors.New("xray unavailable")
	app, events := newLifecycleTestApp(func(name string) error {
		if name == "start xray" {
			return xrayErr
		}
		return nil
	})

	err := app.Start()
	if !errors.Is(err, xrayErr) {
		t.Fatalf("Start() error = %v, want wrapped %v", err, xrayErr)
	}

	want := []string{
		"start cron", "start web", "start sub", "start core", "start xray",
		"stop cron", "stop core", "stop sub", "stop web",
	}
	if got := *events; !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle events = %v, want %v", got, want)
	}
}

func TestRestartAppRestoresRuntimeAfterFailedRestart(t *testing.T) {
	var xrayStarts int
	app, events := newLifecycleTestApp(func(name string) error {
		if name == "start xray" {
			xrayStarts++
			if xrayStarts == 1 {
				return errors.New("transient xray failure")
			}
		}
		return nil
	})

	err := app.RestartApp()
	if err != nil {
		t.Fatalf("RestartApp() error = %v, want nil after successful restore", err)
	}
	if xrayStarts != 2 {
		t.Fatalf("Xray start attempts = %d, want 2 (restart plus rollback)", xrayStarts)
	}

	want := []string{
		"stop cron", "stop sub", "stop web", "stop xray", "stop core",
		"start cron", "start web", "start sub", "start core", "start xray",
		"stop cron", "stop core", "stop sub", "stop web",
		"start cron", "start web", "start sub", "start core", "start xray",
	}
	if got := *events; !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle events = %v, want %v", got, want)
	}
}

func newLifecycleTestApp(fail func(string) error) (*APP, *[]string) {
	events := make([]string, 0, 16)
	record := func(name string) error {
		events = append(events, name)
		return fail(name)
	}
	app := &APP{lifecycle: appLifecycleHooks{
		getTimeLocation:       func() (*time.Location, error) { return time.UTC, nil },
		getTrafficAge:         func() (int, error) { return 30, nil },
		startCron:             func(*time.Location, int) error { return record("start cron") },
		stopCron:              func() { events = append(events, "stop cron") },
		startWeb:              func() error { return record("start web") },
		stopWeb:               func() error { return record("stop web") },
		startSub:              func() error { return record("start sub") },
		stopSub:               func() error { return record("stop sub") },
		startCore:             func() error { return record("start core") },
		stopCore:              func() error { return record("stop core") },
		hasEnabledXrayInbound: func() (bool, error) { return true, nil },
		startXray:             func() error { return record("start xray") },
		stopXray:              func() error { return record("stop xray") },
	}}
	return app, &events
}
