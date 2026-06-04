package sub

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

type SubscriptionCheckReport struct {
	warnings         []string
	dnsSensitiveTags []string
}

func NewSubscriptionCheckReport() *SubscriptionCheckReport {
	return &SubscriptionCheckReport{}
}

func (r *SubscriptionCheckReport) Warnings() []string {
	if r == nil || len(r.warnings) == 0 {
		return nil
	}
	return append([]string{}, r.warnings...)
}

func (r *SubscriptionCheckReport) warn(format string, args ...interface{}) {
	if r == nil {
		return
	}
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}

func (r *SubscriptionCheckReport) CheckProtocolCredentials(protocol string, configs map[string]interface{}, inbound *model.Inbound, outbound map[string]interface{}) {
	if r == nil {
		return
	}
	tag, _ := outbound["tag"].(string)
	switch protocol {
	case "shadowsocks":
		options := inboundOptions(inbound)
		method, _ := options["method"].(string)
		configKey := "shadowsocks"
		if method == "2022-blake3-aes-128-gcm" {
			configKey = "shadowsocks16"
		}
		if method == "" {
			r.warn("shadowsocks inbound %q has empty method", tag)
		}
		if method != "none" && !configHasString(configs, configKey, "password") {
			r.warn("shadowsocks inbound %q requires client config %q.password", tag, configKey)
		}
		if strings.HasPrefix(method, "2022") && !mapHasString(options, "password") {
			r.warn("shadowsocks 2022 inbound %q requires server password for multi-user key derivation", tag)
		}
	case "trojan":
		if !configHasString(configs, "trojan", "password") {
			r.warn("trojan inbound %q requires client config trojan.password", tag)
		}
	case "vless":
		if !configHasString(configs, "vless", "uuid") {
			r.warn("vless inbound %q requires client config vless.uuid", tag)
		}
		if configHasString(configs, "vless", "flow") && inbound.TlsId == 0 {
			r.warn("vless inbound %q has flow configured but TLS is disabled", tag)
		}
	case "vmess":
		if !configHasString(configs, "vmess", "uuid") {
			r.warn("vmess inbound %q requires client config vmess.uuid", tag)
		}
	}
}

func (r *SubscriptionCheckReport) CheckTransportDNSLinkage(protocol string, inbound *model.Inbound, outbound map[string]interface{}) {
	if r == nil {
		return
	}
	tag, _ := outbound["tag"].(string)
	tls, hasTLS := outbound["tls"].(map[string]interface{})
	transport, hasTransport := outbound["transport"].(map[string]interface{})
	if hasTLS {
		if enabled, ok := tls["enabled"].(bool); !ok || enabled {
			r.dnsSensitiveTags = appendUnique(r.dnsSensitiveTags, tag)
			if _, ok := tls["server_name"].(string); !ok {
				r.warn("%s outbound %q uses TLS without explicit server_name; DNS/SNI routing may be ambiguous", protocol, tag)
			}
			if ech, ok := tls["ech"].(map[string]interface{}); ok {
				if enabled, ok := ech["enabled"].(bool); ok && enabled {
					r.dnsSensitiveTags = appendUnique(r.dnsSensitiveTags, tag)
					if len(stringSliceFromValue(ech["config"])) == 0 {
						r.warn("%s outbound %q enables ECH but config is empty", protocol, tag)
					}
				}
			}
			if reality, ok := tls["reality"].(map[string]interface{}); ok {
				if enabled, ok := reality["enabled"].(bool); ok && enabled {
					r.dnsSensitiveTags = appendUnique(r.dnsSensitiveTags, tag)
					if !mapHasString(reality, "public_key") {
						r.warn("%s outbound %q enables Reality but public_key is empty", protocol, tag)
					}
				}
			}
		}
	}
	if !hasTransport {
		return
	}
	transportType, _ := transport["type"].(string)
	switch transportType {
	case "ws", "grpc", "httpupgrade", "http":
		r.dnsSensitiveTags = appendUnique(r.dnsSensitiveTags, tag)
	}
	switch transportType {
	case "ws":
		if !transportHasHost(transport) {
			r.warn("%s outbound %q uses WebSocket without Host/header override", protocol, tag)
		}
		if hasPositiveNumber(transport["max_early_data"]) && !mapHasString(transport, "early_data_header_name") {
			r.warn("%s outbound %q sets WebSocket max_early_data without early_data_header_name", protocol, tag)
		}
	case "grpc":
		if !mapHasString(transport, "service_name") {
			r.warn("%s outbound %q uses gRPC without service_name", protocol, tag)
		}
	case "httpupgrade":
		if !transportHasHost(transport) {
			r.warn("%s outbound %q uses HTTPUpgrade without host/header override", protocol, tag)
		}
	}
}

func (r *SubscriptionCheckReport) CheckUDPProtocol(protocol string, inbound *model.Inbound, outbound map[string]interface{}) {
	if r == nil {
		return
	}
	tag, _ := outbound["tag"].(string)
	switch protocol {
	case "hysteria2":
		if network, _ := outbound["network"].(string); network == "tcp" {
			r.warn("hysteria2 outbound %q is UDP based but network is forced to tcp", tag)
		}
		if _, ok := outbound["tls"].(map[string]interface{}); !ok && inbound.TlsId == 0 {
			r.warn("hysteria2 outbound %q should keep TLS enabled", tag)
		}
		if ports, ok := outbound["server_ports"]; ok && ports != nil {
			if _, ok := outbound["hop_interval"]; !ok {
				r.warn("hysteria2 outbound %q uses server_ports without hop_interval", tag)
			}
		}
	case "tuic":
		if network, _ := outbound["network"].(string); network == "tcp" {
			r.warn("tuic outbound %q is UDP based but network is forced to tcp", tag)
		}
		if _, ok := outbound["tls"].(map[string]interface{}); !ok && inbound.TlsId == 0 {
			r.warn("tuic outbound %q should keep TLS enabled", tag)
		}
		if !mapHasString(outbound, "uuid") || !mapHasString(outbound, "password") {
			r.warn("tuic outbound %q requires both uuid and password", tag)
		}
	}
}

func (r *SubscriptionCheckReport) RunDNSLinkage(jsonConfig map[string]interface{}) {
	if r == nil || len(r.dnsSensitiveTags) == 0 {
		return
	}
	if _, ok := jsonConfig["dns"].(map[string]interface{}); !ok {
		r.warn("DNS-sensitive outbounds %s exist but subscription DNS is disabled", strings.Join(r.dnsSensitiveTags, ", "))
		return
	}
	route, _ := jsonConfig["route"].(map[string]interface{})
	rules, _ := route["rules"].([]interface{})
	if !hasDNSHijackRule(rules) {
		r.warn("DNS-sensitive outbounds %s exist but route.rules has no DNS hijack", strings.Join(r.dnsSensitiveTags, ", "))
	}
	if _, ok := route["default_domain_resolver"].(string); !ok {
		r.warn("DNS-sensitive outbounds %s exist but route.default_domain_resolver is empty", strings.Join(r.dnsSensitiveTags, ", "))
	}
}

func (r *SubscriptionCheckReport) Apply(jsonConfig *map[string]interface{}) {
	if r == nil || len(r.warnings) == 0 {
		return
	}
	for _, warning := range r.warnings {
		logger.Warning("subscription check: ", warning)
	}
}

func inboundOptions(inbound *model.Inbound) map[string]interface{} {
	options := map[string]interface{}{}
	if inbound == nil || len(inbound.Options) == 0 {
		return options
	}
	_ = json.Unmarshal(inbound.Options, &options)
	return options
}

func configHasString(configs map[string]interface{}, key string, field string) bool {
	config, ok := configs[key].(map[string]interface{})
	if !ok {
		return false
	}
	return mapHasString(config, field)
}

func mapHasString(data map[string]interface{}, key string) bool {
	value, _ := data[key].(string)
	return value != ""
}

func stringSliceFromValue(value interface{}) []string {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

func hasPositiveNumber(value interface{}) bool {
	switch typed := value.(type) {
	case int:
		return typed > 0
	case float64:
		return typed > 0
	default:
		return false
	}
}

func transportHasHost(transport map[string]interface{}) bool {
	if mapHasString(transport, "host") {
		return true
	}
	if hosts, ok := transport["host"].([]interface{}); ok && len(hosts) > 0 {
		return true
	}
	headers, ok := transport["headers"].(map[string]interface{})
	if !ok {
		return false
	}
	return mapHasString(headers, "Host") || mapHasString(headers, "host")
}

func hasDNSHijackRule(rules []interface{}) bool {
	tcpCovered := false
	udpCovered := false
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok || rule["action"] != "hijack-dns" {
			continue
		}
		if containsRuleValue(rule["protocol"], "dns") {
			return true
		}
		if !containsRuleNumber(rule["port"], 53) {
			continue
		}
		network := rule["network"]
		if network == nil {
			return true
		}
		if containsRuleValue(network, "tcp") {
			tcpCovered = true
		}
		if containsRuleValue(network, "udp") {
			udpCovered = true
		}
		if tcpCovered && udpCovered {
			return true
		}
	}
	return false
}

func containsRuleValue(value interface{}, expected string) bool {
	switch typed := value.(type) {
	case string:
		return typed == expected
	case []interface{}:
		for _, item := range typed {
			if str, ok := item.(string); ok && str == expected {
				return true
			}
		}
	}
	return false
}

func containsRuleNumber(value interface{}, expected int) bool {
	switch typed := value.(type) {
	case int:
		return typed == expected
	case float64:
		return typed == float64(expected)
	case []interface{}:
		for _, item := range typed {
			if containsRuleNumber(item, expected) {
				return true
			}
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
