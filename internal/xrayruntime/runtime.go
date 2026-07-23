// Package xrayruntime embeds the Xray features used by S-UI directly in the
// main Go process. It intentionally exposes only the small lifecycle surface
// needed by the Reality TCP Vision and XHTTP configuration service.
package xrayruntime

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	xraycore "github.com/xtls/xray-core/core"
	xraytls "github.com/xtls/xray-core/transport/internet/tls"
)

// Server is a prepared in-process Xray instance.
type Server struct {
	mu      sync.Mutex
	server  xraycore.Server
	started bool
	retired bool
}

var processInstance struct {
	sync.Mutex
	server *Server
}

// Validate parses an Xray JSON configuration without constructing an Xray
// instance. It is safe to call while the current embedded instance is running.
func Validate(config []byte) error {
	if _, err := xraycore.LoadConfig("json", bytes.NewReader(config)); err != nil {
		return fmt.Errorf("load embedded Xray config: %w", err)
	}
	return nil
}

// New parses an Xray JSON configuration and constructs the single Xray
// instance owned by this process. Xray-core has process-wide dialer state, so
// even an unstarted second instance is not allowed alongside the current one.
func New(config []byte) (*Server, error) {
	processInstance.Lock()
	defer processInstance.Unlock()
	if processInstance.server != nil {
		return nil, fmt.Errorf("an embedded Xray instance already exists in this process")
	}
	parsed, err := xraycore.LoadConfig("json", bytes.NewReader(config))
	if err != nil {
		return nil, fmt.Errorf("load embedded Xray config: %w", err)
	}
	instance, err := xraycore.New(parsed)
	if err != nil {
		return nil, fmt.Errorf("create embedded Xray instance: %w", err)
	}
	server := &Server{server: instance}
	processInstance.server = server
	return server, nil
}

// Start opens all configured listeners.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return fmt.Errorf("embedded Xray instance is closed")
	}
	if s.retired {
		return fmt.Errorf("embedded Xray instance is retired")
	}
	if s.started {
		return nil
	}
	instance := s.server
	if err := instance.Start(); err != nil {
		// Xray documents the instance state as unknown after Start fails and
		// does not support starting an instance more than once. Retire this
		// instance and make a best-effort cleanup instead of allowing a retry
		// against partially started features/listeners.
		s.retired = true
		s.started = false
		startErr := fmt.Errorf("start embedded Xray instance: %w", err)
		if closeErr := instance.Close(); closeErr != nil {
			return errors.Join(startErr, fmt.Errorf("close failed embedded Xray instance: %w", closeErr))
		}
		s.server = nil
		releaseProcessInstance(s)
		return startErr
	}
	s.started = true
	return nil
}

// Close stops all listeners and releases the instance.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return nil
	}
	instance := s.server
	err := instance.Close()
	if err != nil {
		return fmt.Errorf("close embedded Xray instance: %w", err)
	}
	s.server = nil
	s.started = false
	s.retired = true
	releaseProcessInstance(s)
	return nil
}

func releaseProcessInstance(server *Server) {
	processInstance.Lock()
	defer processInstance.Unlock()
	if processInstance.server == server {
		processInstance.server = nil
	}
}

// Version reports the source version compiled into S-UI.
func Version() string {
	return xraycore.Version()
}

// ValidFingerprint reports whether the embedded Xray version can construct
// the uTLS fingerprint written into generated client subscriptions.
func ValidFingerprint(name string) bool {
	return xraytls.GetFingerprint(name) != nil
}
