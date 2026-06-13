package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/util/common"
)

var InboundTypeWithLink = []string{"socks", "http", "mixed", "shadowsocks", "naive", "hysteria", "hysteria2", "anytls", "tuic", "vless", "trojan", "vmess"}

type LinkParam struct {
	Key   string
	Value string
}

const AutoServerKey = "auto_server"

var seededDefaultInboundTagPrefixes = []string{
	"vless-reality-",
	"hy2-",
	"tuic-",
	"trojan-tls-",
	"vmess-ws-tls-",
	"ss-2022-",
	"naive-tls-",
	"anytls-",
	"hysteria-tls-",
}

func IsSeededDefaultInboundTag(tag string) bool {
	for _, prefix := range seededDefaultInboundTagPrefixes {
		if strings.HasPrefix(tag, prefix) {
			return true
		}
	}
	return false
}

func ResolveAddrServer(addr map[string]interface{}, hostname string, inboundTag string) string {
	hostname = strings.TrimSpace(hostname)
	server, _ := addr["server"].(string)
	server = strings.TrimSpace(server)
	if strings.HasPrefix(server, "[") && strings.HasSuffix(server, "]") {
		server = strings.Trim(server, "[]")
	}

	autoServer, _ := boolValue(addr, AutoServerKey)
	if hostname != "" && (autoServer || server == "" || IsSeededDefaultInboundTag(inboundTag)) {
		if strings.HasPrefix(hostname, "[") && strings.HasSuffix(hostname, "]") {
			return strings.Trim(hostname, "[]")
		}
		return hostname
	}
	return server
}

func FormatLinkHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" || strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host
	}
	if strings.Contains(host, ":") {
		return "[" + strings.Trim(host, "[]") + "]"
	}
	return host
}

func addrServerForLink(addr map[string]interface{}, hostname string, inboundTag string) string {
	return FormatLinkHost(ResolveAddrServer(addr, hostname, inboundTag))
}

func AddrServerPort(addr map[string]interface{}) float64 {
	switch port := addr["server_port"].(type) {
	case float64:
		return port
	case float32:
		return float64(port)
	case int:
		return float64(port)
	case int64:
		return float64(port)
	case uint:
		return float64(port)
	case uint64:
		return float64(port)
	case json.Number:
		parsed, _ := strconv.ParseFloat(string(port), 64)
		return parsed
	default:
		return 0
	}
}

func LinkGenerator(clientConfig json.RawMessage, i *model.Inbound, hostname string) []string {
	inbound, err := i.MarshalFull()
	if err != nil {
		return []string{}
	}

	var tls map[string]interface{}
	if i.TlsId > 0 {
		tls = prepareTls(i.Tls)
	}

	var userConfig map[string]map[string]interface{}
	if err := json.Unmarshal(clientConfig, &userConfig); err != nil {
		return []string{}
	}

	var Addrs []map[string]interface{}
	if err := json.Unmarshal(i.Addrs, &Addrs); err != nil {
		return []string{}
	}
	if len(Addrs) == 0 {
		Addrs = append(Addrs, map[string]interface{}{
			"server":      hostname,
			"server_port": (*inbound)["listen_port"],
			"remark":      i.Tag,
		})
		if i.TlsId > 0 {
			Addrs[0]["tls"] = tls
		}
	} else {
		for index, addr := range Addrs {
			addrRemark, _ := addr["remark"].(string)
			Addrs[index]["remark"] = i.Tag + addrRemark
			if i.TlsId > 0 {
				newTls := map[string]interface{}{}
				for k, v := range tls {
					newTls[k] = v
				}

				// Override tls
				if addrTls, ok := addr["tls"].(map[string]interface{}); ok {
					for k, v := range addrTls {
						newTls[k] = v
					}
				}
				Addrs[index]["tls"] = newTls
			}
		}
	}
	for index := range Addrs {
		Addrs[index]["server"] = ResolveAddrServer(Addrs[index], hostname, i.Tag)
	}

	switch i.Type {
	case "socks":
		return socksLink(userConfig["socks"], Addrs)
	case "http":
		return httpLink(userConfig["http"], Addrs)
	case "mixed":
		return append(
			socksLink(userConfig["socks"], Addrs),
			httpLink(userConfig["http"], Addrs)...,
		)
	case "shadowsocks":
		return shadowsocksLink(userConfig, *inbound, Addrs)
	case "naive":
		return naiveLink(userConfig["naive"], *inbound, Addrs)
	case "hysteria":
		return hysteriaLink(userConfig["hysteria"], *inbound, Addrs)
	case "hysteria2":
		return hysteria2Link(userConfig["hysteria2"], *inbound, Addrs)
	case "tuic":
		return tuicLink(userConfig["tuic"], *inbound, Addrs)
	case "vless":
		return vlessLink(userConfig["vless"], *inbound, Addrs)
	case "anytls":
		return anytlsLink(userConfig["anytls"], Addrs)
	case "trojan":
		return trojanLink(userConfig["trojan"], *inbound, Addrs)
	case "vmess":
		return vmessLink(userConfig["vmess"], *inbound, Addrs)
	}

	return []string{}
}

func prepareTls(t *model.Tls) map[string]interface{} {
	if t == nil {
		return nil
	}
	var iTls, oTls map[string]interface{}
	if err := json.Unmarshal(t.Client, &oTls); err != nil {
		return nil
	}
	if err := json.Unmarshal(t.Server, &iTls); err != nil {
		return nil
	}

	for k, v := range iTls {
		switch k {
		case "enabled", "server_name", "alpn":
			oTls[k] = v
		case "reality":
			reality, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			clientReality, ok := oTls["reality"].(map[string]interface{})
			if !ok {
				clientReality = map[string]interface{}{}
			}
			clientReality["enabled"] = reality["enabled"]
			if shortIDs, hasSIds := reality["short_id"].([]interface{}); hasSIds && len(shortIDs) > 0 {
				clientReality["short_id"] = shortIDs[common.RandomInt(len(shortIDs))]
			}
			oTls["reality"] = clientReality
		}
	}
	return oTls
}

func socksLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	for _, addr := range addrs {
		links = append(links, fmt.Sprintf("socks5://%s:%s@%s:%d", userConfig["username"], userConfig["password"], addrServerForLink(addr, "", ""), uint(AddrServerPort(addr))))
	}
	return links
}

func httpLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	protocol := "http"
	for _, addr := range addrs {
		if addr["tls"] != nil {
			protocol = "https"
		}
		links = append(links, fmt.Sprintf("%s://%s:%s@%s:%d", protocol, userConfig["username"], userConfig["password"], addrServerForLink(addr, "", ""), uint(AddrServerPort(addr))))
	}
	return links
}

func shadowsocksLink(
	userConfig map[string]map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	var userPass []string
	method, _ := inbound["method"].(string)
	if strings.HasPrefix(method, "2022") {
		inbPass, _ := inbound["password"].(string)
		userPass = append(userPass, inbPass)
	}
	var pass string
	if method == "2022-blake3-aes-128-gcm" {
		pass, _ = userConfig["shadowsocks16"]["password"].(string)
	} else {
		pass, _ = userConfig["shadowsocks"]["password"].(string)
	}
	userPass = append(userPass, pass)

	uriBase := fmt.Sprintf("ss://%s", toBase64([]byte(fmt.Sprintf("%s:%s", method, strings.Join(userPass, ":")))))

	var links []string
	for _, addr := range addrs {
		port := AddrServerPort(addr)
		links = append(links, fmt.Sprintf("%s@%s:%.0f#%s", uriBase, addrServerForLink(addr, "", ""), port, addr["remark"].(string)))
	}
	return links
}

func naiveLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	username, _ := userConfig["username"].(string)

	baseUri := "http2://"
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		params = append(params, LinkParam{"padding", "1"})
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if sni, ok := tls["server_name"].(string); ok {
				params = append(params, LinkParam{"peer", sni})
			}
			if alpn := stringSliceValue(tls["alpn"]); len(alpn) > 0 {
				params = append(params, LinkParam{"alpn", strings.Join(alpn, ",")})
			}
			if insecure, ok := tls["insecure"].(bool); ok && insecure {
				params = append(params, LinkParam{"insecure", "1"})
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"tfo", "1"})
		} else {
			params = append(params, LinkParam{"tfo", "0"})
		}

		port := AddrServerPort(addr)
		uri := baseUri + toBase64([]byte(fmt.Sprintf("%s:%s@%s:%.0f", username, password, addrServerForLink(addr, "", ""), port)))
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}
	return links
}

func hysteriaLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	baseUri := "hysteria://"
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params = append(params, LinkParam{"downmbps", fmt.Sprintf("%.0f", upmbps)})
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params = append(params, LinkParam{"upmbps", fmt.Sprintf("%.0f", downmbps)})
		}
		if auth, ok := userConfig["auth_str"].(string); ok {
			params = append(params, LinkParam{"auth", auth})
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(string); ok {
			params = append(params, LinkParam{"obfs", obfs})
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"fastopen", "1"})
		} else {
			params = append(params, LinkParam{"fastopen", "0"})
		}
		var outJson map[string]interface{}
		if err := json.Unmarshal(inbound["out_json"].(json.RawMessage), &outJson); err != nil {
			return []string{} // Handle error
		}
		if mport := stringSliceValue(outJson["server_ports"]); len(mport) > 0 {
			params = append(params, LinkParam{"mport", strings.Join(mport, ",")})
		}

		port := AddrServerPort(addr)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addrServerForLink(addr, "", ""), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func hysteria2Link(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	baseUri := fmt.Sprintf("%s%s@", "hysteria2://", password)
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params = append(params, LinkParam{"downmbps", fmt.Sprintf("%.0f", upmbps)})
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params = append(params, LinkParam{"upmbps", fmt.Sprintf("%.0f", downmbps)})
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(map[string]interface{}); ok {
			if obfsType, ok := obfs["type"].(string); ok {
				params = append(params, LinkParam{"obfs", obfsType})
			}
			if obfsPassword, ok := obfs["password"].(string); ok {
				params = append(params, LinkParam{"obfs-password", obfsPassword})
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"fastopen", "1"})
		} else {
			params = append(params, LinkParam{"fastopen", "0"})
		}
		var outJson map[string]interface{}
		if err := json.Unmarshal(inbound["out_json"].(json.RawMessage), &outJson); err != nil {
			return []string{} // Handle error
		}
		if mport := stringSliceValue(outJson["server_ports"]); len(mport) > 0 {
			params = append(params, LinkParam{"mport", strings.Join(mport, ",")})
		}

		port := AddrServerPort(addr)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addrServerForLink(addr, "", ""), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func anytlsLink(
	userConfig map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	baseUri := fmt.Sprintf("%s%s@", "anytls://", password)
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}

		port := AddrServerPort(addr)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addrServerForLink(addr, "", ""), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func tuicLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	uuid, _ := userConfig["uuid"].(string)
	baseUri := fmt.Sprintf("%s%s:%s@", "tuic://", uuid, password)
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if congestionControl, ok := inbound["congestion_control"].(string); ok {
			params = append(params, LinkParam{"congestion_control", congestionControl})
		}

		port := AddrServerPort(addr)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addrServerForLink(addr, "", ""), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func vlessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok && enabledValue(tls, "enabled", false) {
			getTlsParams(&params, tls, "allowInsecure")
			if flow, ok := userConfig["flow"].(string); ok {
				params = append(params, LinkParam{"flow", flow})
			}
		}
		port := AddrServerPort(addr)
		uri := fmt.Sprintf("vless://%s@%s:%.0f", uuid, addrServerForLink(addr, "", ""), port)
		uri = addParams(uri, params, addr["remark"].(string))
		links = append(links, uri)
	}

	return links
}

func trojanLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {
	password, _ := userConfig["password"].(string)
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok && enabledValue(tls, "enabled", false) {
			getTlsParams(&params, tls, "allowInsecure")
		}
		port := AddrServerPort(addr)
		uri := fmt.Sprintf("trojan://%s@%s:%.0f", password, addrServerForLink(addr, "", ""), port)
		uri = addParams(uri, params, addr["remark"].(string))
		links = append(links, uri)
	}

	return links
}

func vmessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	transportParams := getTransportParams(inbound["transport"])
	var links []string

	baseParams := map[string]interface{}{
		"v":   "2",
		"id":  uuid,
		"aid": 0,
	}

	var net, typ, host, path string
	for _, p := range transportParams {
		switch p.Key {
		case "type":
			net = p.Value
		case "host":
			host = p.Value
		case "path":
			path = p.Value
		}
	}

	if net == "http" || net == "tcp" {
		baseParams["net"] = "tcp"
		if net == "http" {
			typ = "http"
		}
	} else {
		baseParams["net"] = net
	}

	for _, addr := range addrs {
		obj := make(map[string]interface{})
		for k, v := range baseParams {
			obj[k] = v
		}

		obj["add"] = ResolveAddrServer(addr, "", "")
		port := AddrServerPort(addr)
		obj["port"] = fmt.Sprintf("%.0f", port)
		obj["ps"], _ = addr["remark"].(string)
		if typ != "" {
			obj["type"] = typ
		}
		if host != "" {
			obj["host"] = host
		}
		if path != "" {
			obj["path"] = path
		}
		populateVmessTlsParams(obj, addr["tls"])

		jsonStr, _ := json.Marshal(obj)

		uri := fmt.Sprintf("vmess://%s", toBase64(jsonStr))
		links = append(links, uri)
	}
	return links
}

func populateVmessTlsParams(obj map[string]interface{}, tlsConfig interface{}) {
	if tlsMap, ok := tlsConfig.(map[string]interface{}); ok && enabledValue(tlsMap, "enabled", false) {
		obj["tls"] = "tls"
		var tlsParams []LinkParam
		getTlsParams(&tlsParams, tlsMap, "allowInsecure")
		for _, p := range tlsParams {
			switch p.Key {
			case "security":
				// ignore, as "tls" is already set
			case "allowInsecure":
				obj["allowInsecure"] = "1"
			case "sni":
				obj["sni"] = p.Value
			case "fp":
				obj["fp"] = p.Value
			case "alpn":
				obj["alpn"] = p.Value
			}
		}
	} else {
		obj["tls"] = "none"
	}
}

func toBase64(d []byte) string {
	return base64.StdEncoding.EncodeToString(d)
}

func addParams(uri string, params []LinkParam, remark string) string {
	URL, _ := url.Parse(uri)
	var q []string
	for _, p := range params {
		switch p.Key {
		case "mport", "alpn":
			q = append(q, fmt.Sprintf("%s=%s", p.Key, p.Value))
		default:
			q = append(q, fmt.Sprintf("%s=%s", p.Key, url.QueryEscape(p.Value)))
		}
	}
	URL.RawQuery = strings.Join(q, "&")
	URL.Fragment = remark
	return URL.String()
}

func getTransportParams(t interface{}) []LinkParam {
	var params []LinkParam
	trasport, _ := t.(map[string]interface{})
	var transportType string
	if tt, ok := trasport["type"].(string); ok {
		transportType = tt
	} else {
		transportType = "tcp"
	}
	params = append(params, LinkParam{"type", transportType})
	if transportType == "tcp" {
		return params
	}

	switch transportType {
	case "http":
		if host, ok := trasport["host"].([]interface{}); ok {
			var hosts []string
			for _, v := range host {
				if hostValue, ok := v.(string); ok && hostValue != "" {
					hosts = append(hosts, hostValue)
				}
			}
			if len(hosts) > 0 {
				params = append(params, LinkParam{"host", strings.Join(hosts, ",")})
			}
		}
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
	case "ws":
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
		if headers, ok := trasport["headers"].(map[string]interface{}); ok {
			if host, ok := headers["Host"].(string); ok {
				params = append(params, LinkParam{"host", host})
			}
		}
	case "grpc":
		if serviceName, ok := trasport["service_name"].(string); ok {
			params = append(params, LinkParam{"serviceName", serviceName})
		}
	case "httpupgrade":
		if host, ok := trasport["host"].(string); ok {
			params = append(params, LinkParam{"host", host})
		}
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
	}
	return params
}

func getTlsParams(params *[]LinkParam, tls map[string]interface{}, insecureKey string) {
	if reality, ok := tls["reality"].(map[string]interface{}); ok && enabledValue(reality, "enabled", false) {
		*params = append(*params, LinkParam{"security", "reality"})
		if pbk, ok := reality["public_key"].(string); ok {
			*params = append(*params, LinkParam{"pbk", pbk})
		}
		if sid, ok := reality["short_id"].(string); ok {
			*params = append(*params, LinkParam{"sid", sid})
		}
	} else {
		*params = append(*params, LinkParam{"security", "tls"})
		if insecure, ok := tls["insecure"].(bool); ok && insecure {
			*params = append(*params, LinkParam{insecureKey, "1"})
		}
		if disableSni, ok := tls["disable_sni"].(bool); ok && disableSni {
			*params = append(*params, LinkParam{"disable_sni", "1"})
		}
	}
	if utls, ok := tls["utls"].(map[string]interface{}); ok {
		if fingerprint, ok := utls["fingerprint"].(string); ok {
			*params = append(*params, LinkParam{"fp", fingerprint})
		}
	}
	if sni, ok := tls["server_name"].(string); ok {
		*params = append(*params, LinkParam{"sni", sni})
	}
	if alpn, ok := tls["alpn"].([]interface{}); ok {
		alpnList := make([]string, 0, len(alpn))
		for _, v := range alpn {
			if alpnValue, ok := v.(string); ok && alpnValue != "" {
				alpnList = append(alpnList, alpnValue)
			}
		}
		if len(alpnList) > 0 {
			*params = append(*params, LinkParam{"alpn", strings.Join(alpnList, ",")})
		}
	}
}
