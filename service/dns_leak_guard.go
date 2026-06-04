package service

import (
	"encoding/json"
	"fmt"
)

const (
	remoteDNSTag                = "remote-dns"
	localDNSTag                 = "local-dns"
	dnsHijackAction             = "hijack-dns"
	DNSLeakGuardModeOff         = "off"
	DNSLeakGuardModeRecommended = "recommended"
	DNSLeakGuardModeStrict      = "strict"
)

type DNSLeakGuardCheckReport struct {
	Mode    string              `json:"mode"`
	Enabled bool                `json:"enabled"`
	Passed  bool                `json:"passed"`
	Status  string              `json:"status"`
	Checks  []DNSLeakGuardCheck `json:"checks"`
}

type DNSLeakGuardCheck struct {
	Key      string `json:"key"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func BuildDNSLeakGuardCheckReport(config json.RawMessage, mode string) (*DNSLeakGuardCheckReport, error) {
	mode = NormalizeDNSLeakGuardMode(mode)
	report := &DNSLeakGuardCheckReport{
		Mode:    mode,
		Enabled: mode != DNSLeakGuardModeOff,
		Passed:  mode != DNSLeakGuardModeOff,
		Status:  "passed",
		Checks:  []DNSLeakGuardCheck{},
	}
	if mode == DNSLeakGuardModeOff {
		report.Passed = false
		report.Status = "warning"
		report.Checks = append(report.Checks, DNSLeakGuardCheck{
			Key:      "disabled",
			Passed:   false,
			Severity: "warning",
			Message:  "DNS leak guard is disabled",
		})
		return report, nil
	}

	var coreConfig map[string]json.RawMessage
	if err := json.Unmarshal(config, &coreConfig); err != nil {
		return nil, err
	}
	dnsConfig, err := decodeObjectSection("dns", coreConfig["dns"])
	if err != nil {
		return nil, err
	}
	routeConfig, err := decodeObjectSection("route", coreConfig["route"])
	if err != nil {
		return nil, err
	}

	servers, err := getObjectSlice(dnsConfig, "servers")
	if err != nil {
		return nil, err
	}
	serverTags, err := dnsServerTags(servers)
	if err != nil {
		return nil, err
	}
	remoteDNS := findDNSServer(servers, remoteDNSTag)
	localDNS := findDNSServer(servers, localDNSTag)

	addDNSCheck(report, "remoteDns", remoteDNS != nil, "error", "remote-dns server exists")
	addDNSCheck(report, "remoteDnsEncrypted", remoteDNS != nil && isEncryptedDNSType(remoteDNS["type"]), "error", "remote-dns uses encrypted DNS")
	addDNSCheck(report, "localDns", localDNS != nil, "warning", "local-dns server exists")

	final, _ := dnsConfig["final"].(string)
	addDNSCheck(report, "finalDns", final == remoteDNSTag && serverTags[remoteDNSTag], "error", "dns.final points to remote-dns")

	rules, err := getObjectSlice(dnsConfig, "rules")
	if err != nil {
		return nil, err
	}
	addDNSCheck(report, "localRule", hasLocalDNSRule(rules), "warning", "local DNS rule routes LAN domains to local-dns")

	routeRules, err := getObjectSlice(routeConfig, "rules")
	if err != nil {
		return nil, err
	}
	addDNSCheck(report, "protocolHijack", hasProtocolDNSHijack(routeRules), "error", "DNS protocol traffic is hijacked")
	addDNSCheck(report, "port53Hijack", hasPort53Hijack(routeRules), "error", "TCP/UDP port 53 traffic is hijacked")
	autoDetect, _ := routeConfig["auto_detect_interface"].(bool)
	addDNSCheck(report, "autoDetectInterface", autoDetect, "warning", "route.auto_detect_interface is enabled")
	if mode == DNSLeakGuardModeStrict {
		resolver, _ := routeConfig["default_domain_resolver"].(string)
		addDNSCheck(report, "strictResolver", resolver == remoteDNSTag, "error", "strict mode uses remote-dns as default domain resolver")
	}

	return report, nil
}

func addDNSCheck(report *DNSLeakGuardCheckReport, key string, passed bool, severity string, message string) {
	report.Checks = append(report.Checks, DNSLeakGuardCheck{
		Key:      key,
		Passed:   passed,
		Severity: severity,
		Message:  message,
	})
	if passed {
		return
	}
	if severity == "error" {
		report.Passed = false
		report.Status = "failed"
	} else if report.Status != "failed" {
		report.Status = "warning"
	}
}

func findDNSServer(servers []interface{}, tag string) map[string]interface{} {
	for _, item := range servers {
		server, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if serverTag, _ := server["tag"].(string); serverTag == tag {
			return server
		}
	}
	return nil
}

func isEncryptedDNSType(value interface{}) bool {
	switch value {
	case "tls", "https", "quic", "h3":
		return true
	default:
		return false
	}
}

func hasLocalDNSRule(rules []interface{}) bool {
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		server, _ := rule["server"].(string)
		if server != localDNSTag {
			continue
		}
		if containsString(rule["domain_suffix"], ".lan") && containsString(rule["domain_suffix"], ".local") {
			return true
		}
	}
	return false
}

func NormalizeDNSLeakGuardMode(mode string) string {
	switch mode {
	case DNSLeakGuardModeOff, DNSLeakGuardModeStrict:
		return mode
	default:
		return DNSLeakGuardModeRecommended
	}
}

// DefaultDNSLeakGuardConfig returns a fresh sing-box DNS configuration that avoids
// falling back to the system resolver by default. The detour parameter is kept
// explicit because server-side core config and client subscription config may
// choose different bootstrap policies.
func DefaultDNSLeakGuardConfig(remoteDetour string) map[string]interface{} {
	return map[string]interface{}{
		"servers": []interface{}{
			defaultRemoteDNSServer(remoteDetour),
			defaultLocalDNSServer(),
		},
		"rules": []interface{}{
			defaultLocalDNSRule(),
		},
		"final":             remoteDNSTag,
		"strategy":          "prefer_ipv4",
		"disable_cache":     false,
		"disable_expire":    false,
		"independent_cache": true,
		"reverse_mapping":   true,
	}
}

// DefaultDNSHijackRouteRules returns route rules that force DNS traffic through
// sing-box's DNS module instead of letting TCP/UDP 53 leave via the system path.
func DefaultDNSHijackRouteRules() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"protocol": []interface{}{"dns"},
			"action":   dnsHijackAction,
		},
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"tcp", "udp"},
			"action":  dnsHijackAction,
		},
	}
}

func ensureCoreDNSLeakGuard(coreConfig map[string]json.RawMessage, mode string) error {
	mode = NormalizeDNSLeakGuardMode(mode)
	if mode == DNSLeakGuardModeOff {
		return nil
	}

	var defaultCoreConfig map[string]json.RawMessage
	if err := json.Unmarshal([]byte(defaultConfig), &defaultCoreConfig); err != nil {
		return err
	}

	dnsRaw := coreConfig["dns"]
	if isEmptyJSON(dnsRaw) {
		dnsRaw = defaultCoreConfig["dns"]
	}
	dnsConfig, err := decodeObjectSection("dns", dnsRaw)
	if err != nil {
		return err
	}
	if err := EnsureDNSLeakGuard(dnsConfig, mode, "direct"); err != nil {
		return err
	}
	coreConfig["dns"], err = json.Marshal(dnsConfig)
	if err != nil {
		return err
	}

	routeRaw := coreConfig["route"]
	if isEmptyJSON(routeRaw) {
		routeRaw = defaultCoreConfig["route"]
	}
	routeConfig, err := decodeObjectSection("route", routeRaw)
	if err != nil {
		return err
	}
	if err := EnsureRouteDNSHijack(routeConfig, mode); err != nil {
		return err
	}
	if err := ValidateRouteDefaultDomainResolver(dnsConfig, routeConfig); err != nil {
		return err
	}
	coreConfig["route"], err = json.Marshal(routeConfig)
	return err
}

func EnsureDNSLeakGuard(dnsConfig map[string]interface{}, mode string, remoteDetour string) error {
	mode = NormalizeDNSLeakGuardMode(mode)
	servers, err := getObjectSlice(dnsConfig, "servers")
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		servers = DefaultDNSLeakGuardConfig(remoteDetour)["servers"].([]interface{})
	}

	serverTags, err := dnsServerTags(servers)
	if err != nil {
		return err
	}
	if !serverTags[remoteDNSTag] {
		servers = append(servers, defaultRemoteDNSServer(remoteDetour))
	}
	if !serverTags[localDNSTag] {
		servers = append(servers, defaultLocalDNSServer())
	}
	dnsConfig["servers"] = servers

	serverTags, err = dnsServerTags(servers)
	if err != nil {
		return err
	}
	final, _ := dnsConfig["final"].(string)
	if mode == DNSLeakGuardModeStrict || final == "" || !serverTags[final] {
		dnsConfig["final"] = remoteDNSTag
	}
	strategy, _ := dnsConfig["strategy"].(string)
	if strategy == "" {
		dnsConfig["strategy"] = "prefer_ipv4"
	}
	if _, ok := dnsConfig["disable_cache"]; !ok {
		dnsConfig["disable_cache"] = false
	}
	if _, ok := dnsConfig["disable_expire"]; !ok {
		dnsConfig["disable_expire"] = false
	}
	if _, ok := dnsConfig["independent_cache"]; !ok {
		dnsConfig["independent_cache"] = true
	}
	if _, ok := dnsConfig["reverse_mapping"]; !ok {
		dnsConfig["reverse_mapping"] = true
	}

	rules, err := getObjectSlice(dnsConfig, "rules")
	if err != nil {
		return err
	}
	rules = ensureLocalDNSRule(rules)
	dnsConfig["rules"] = rules

	return validateDNSRuleServers(rules, serverTags)
}

func EnsureRouteDNSHijack(routeConfig map[string]interface{}, mode string) error {
	mode = NormalizeDNSLeakGuardMode(mode)
	rules, err := getObjectSlice(routeConfig, "rules")
	if err != nil {
		return err
	}
	var additions []interface{}
	if !hasProtocolDNSHijack(rules) {
		additions = append(additions, DefaultDNSHijackRouteRules()[0])
	}
	if !hasPort53Hijack(rules) {
		additions = append(additions, DefaultDNSHijackRouteRules()[1])
	}
	if len(additions) > 0 {
		rules = insertAfterSniff(rules, additions...)
	}
	routeConfig["rules"] = rules
	if _, ok := routeConfig["auto_detect_interface"]; !ok {
		routeConfig["auto_detect_interface"] = true
	}
	if mode == DNSLeakGuardModeStrict {
		routeConfig["auto_detect_interface"] = true
		routeConfig["default_domain_resolver"] = remoteDNSTag
	} else if mode == DNSLeakGuardModeRecommended {
		if resolver, _ := routeConfig["default_domain_resolver"].(string); resolver == "" {
			routeConfig["default_domain_resolver"] = remoteDNSTag
		}
	}
	return nil
}

func ValidateRouteDefaultDomainResolver(dnsConfig map[string]interface{}, routeConfig map[string]interface{}) error {
	resolver, _ := routeConfig["default_domain_resolver"].(string)
	if resolver == "" {
		return nil
	}
	servers, err := getObjectSlice(dnsConfig, "servers")
	if err != nil {
		return err
	}
	serverTags, err := dnsServerTags(servers)
	if err != nil {
		return err
	}
	if !serverTags[resolver] {
		return fmt.Errorf("route.default_domain_resolver references unknown DNS server: %s", resolver)
	}
	return nil
}

func decodeObjectSection(name string, raw json.RawMessage) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("invalid %s config: %w", name, err)
	}
	if result == nil {
		return nil, fmt.Errorf("%s config must be a JSON object", name)
	}
	return result, nil
}

func getObjectSlice(obj map[string]interface{}, key string) ([]interface{}, error) {
	value, ok := obj[key]
	if !ok || value == nil {
		return []interface{}{}, nil
	}
	items, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	return items, nil
}

func dnsServerTags(servers []interface{}) (map[string]bool, error) {
	tags := make(map[string]bool, len(servers))
	for _, item := range servers {
		server, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("dns.servers must contain JSON objects")
		}
		tag, _ := server["tag"].(string)
		if tag != "" {
			if tags[tag] {
				return nil, fmt.Errorf("duplicate dns server tag: %s", tag)
			}
			tags[tag] = true
		}
	}
	return tags, nil
}

func ensureLocalDNSRule(rules []interface{}) []interface{} {
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		server, _ := rule["server"].(string)
		if server != localDNSTag {
			continue
		}
		rule["action"] = "route"
		rule["domain_suffix"] = ensureStringValues(rule["domain_suffix"], ".lan", ".local")
		return rules
	}
	return append([]interface{}{defaultLocalDNSRule()}, rules...)
}

func validateDNSRuleServers(rules []interface{}, serverTags map[string]bool) error {
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("dns.rules must contain JSON objects")
		}
		server, _ := rule["server"].(string)
		if server != "" && !serverTags[server] {
			return fmt.Errorf("dns rule references unknown server: %s", server)
		}
		if nested, ok := rule["rules"].([]interface{}); ok {
			if err := validateDNSRuleServers(nested, serverTags); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasProtocolDNSHijack(rules []interface{}) bool {
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if rule["action"] == dnsHijackAction && containsString(rule["protocol"], "dns") {
			return true
		}
	}
	return false
}

func hasPort53Hijack(rules []interface{}) bool {
	tcpCovered := false
	udpCovered := false
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if rule["action"] != dnsHijackAction || !containsNumber(rule["port"], 53) {
			continue
		}
		network := rule["network"]
		if network == nil {
			return true
		}
		if containsString(network, "tcp") {
			tcpCovered = true
		}
		if containsString(network, "udp") {
			udpCovered = true
		}
		if tcpCovered && udpCovered {
			return true
		}
	}
	return false
}

func insertAfterSniff(rules []interface{}, additions ...interface{}) []interface{} {
	if len(rules) > 0 {
		if first, ok := rules[0].(map[string]interface{}); ok && first["action"] == "sniff" {
			result := []interface{}{rules[0]}
			result = append(result, additions...)
			return append(result, rules[1:]...)
		}
	}
	return append(additions, rules...)
}

func containsString(value interface{}, target string) bool {
	switch typed := value.(type) {
	case string:
		return typed == target
	case []interface{}:
		for _, item := range typed {
			if str, ok := item.(string); ok && str == target {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if item == target {
				return true
			}
		}
	}
	return false
}

func ensureStringValues(value interface{}, required ...string) []interface{} {
	values := make([]interface{}, 0)
	seen := map[string]bool{}
	appendValue := func(item string) {
		if item == "" || seen[item] {
			return
		}
		seen[item] = true
		values = append(values, item)
	}
	switch typed := value.(type) {
	case string:
		appendValue(typed)
	case []interface{}:
		for _, item := range typed {
			if str, ok := item.(string); ok {
				appendValue(str)
			}
		}
	case []string:
		for _, item := range typed {
			appendValue(item)
		}
	}
	for _, item := range required {
		appendValue(item)
	}
	return values
}

func containsNumber(value interface{}, target int) bool {
	switch typed := value.(type) {
	case int:
		return typed == target
	case float64:
		return typed == float64(target)
	case []interface{}:
		for _, item := range typed {
			if containsNumber(item, target) {
				return true
			}
		}
	case []int:
		for _, item := range typed {
			if item == target {
				return true
			}
		}
	}
	return false
}

func defaultRemoteDNSServer(detour string) map[string]interface{} {
	if detour == "" {
		detour = "direct"
	}
	return map[string]interface{}{
		"tag":         remoteDNSTag,
		"type":        "tls",
		"server":      "1.1.1.1",
		"server_port": 853,
		"detour":      detour,
		"tls":         map[string]interface{}{},
	}
}

func defaultLocalDNSServer() map[string]interface{} {
	return map[string]interface{}{
		"tag":         localDNSTag,
		"type":        "udp",
		"server":      "223.5.5.5",
		"server_port": 53,
		"detour":      "direct",
	}
}

func defaultLocalDNSRule() map[string]interface{} {
	return map[string]interface{}{
		"domain_suffix": []interface{}{".lan", ".local"},
		"action":        "route",
		"server":        localDNSTag,
	}
}

func isEmptyJSON(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}
