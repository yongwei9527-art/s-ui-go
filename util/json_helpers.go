package util

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

func mapValue(data map[string]interface{}, key string) (map[string]interface{}, bool) {
	value, ok := data[key].(map[string]interface{})
	return value, ok
}

func ensureMapValue(data map[string]interface{}, key string) map[string]interface{} {
	if value, ok := mapValue(data, key); ok {
		return value
	}
	value := map[string]interface{}{}
	data[key] = value
	return value
}

func stringValue(data map[string]interface{}, key string) (string, bool) {
	value, ok := data[key].(string)
	return value, ok && value != ""
}

func boolValue(data map[string]interface{}, key string) (bool, bool) {
	value, ok := data[key].(bool)
	return value, ok
}

func enabledValue(data map[string]interface{}, key string, defaultValue bool) bool {
	value, ok := boolValue(data, key)
	if !ok {
		return defaultValue
	}
	return value
}

func stringSliceValue(value interface{}) []string {
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

func firstQueryValue(q *url.Values, keys ...string) string {
	if q == nil {
		return ""
	}
	for _, key := range keys {
		if value := q.Get(key); value != "" {
			return value
		}
	}
	return ""
}

func queryBool(q *url.Values, keys ...string) bool {
	value := strings.ToLower(firstQueryValue(q, keys...))
	return value == "1" || value == "true" || value == "yes"
}

func queryCSV(q *url.Values, keys ...string) []string {
	value := firstQueryValue(q, keys...)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitHostPortDefault(hostPort string, defaultPort int) (string, int) {
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil || host == "" {
		if parsedHost := strings.Trim(hostPort, "[]"); parsedHost != "" {
			return parsedHost, defaultPort
		}
		return host, defaultPort
	}
	port := defaultPort
	if portStr != "" {
		if parsedPort, err := strconv.Atoi(portStr); err == nil && parsedPort > 0 {
			port = parsedPort
		}
	}
	return host, port
}
