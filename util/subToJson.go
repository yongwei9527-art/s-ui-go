package util

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/util/common"
)

const maxExternalSubscriptionBytes = 8 << 20 // 8 MiB

var externalSubscriptionTimeout = 15 * time.Second

func newExternalSubscriptionClient() *http.Client {
	// Clone the default transport so subscription retrieval honours standard
	// HTTP(S)_PROXY / NO_PROXY settings instead of silently bypassing them.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &http.Client{
		Transport: transport,
		Timeout:   externalSubscriptionTimeout,
	}
}

func GetExternalLink(url string) string {
	ctx, cancel := context.WithTimeout(context.Background(), externalSubscriptionTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Warning("sub: Error creating HTTP request:", err)
		return ""
	}

	response, err := newExternalSubscriptionClient().Do(req)
	if err != nil {
		logger.Warning("sub: Error making HTTP request:", err)
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		logger.Warning("sub: External subscription returned HTTP status:", response.Status)
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxExternalSubscriptionBytes+1))
	if err != nil {
		logger.Warning("sub: Error reading response body:", err)
		return ""
	}
	if len(body) > maxExternalSubscriptionBytes {
		logger.Warning("sub: External subscription exceeds maximum size")
		return ""
	}

	data := StrOrBase64Encoded(string(body))
	return data
}

func GetExternalSub(url string) ([]map[string]interface{}, error) {
	var err error
	var result []map[string]interface{}

	if len(url) == 0 {
		return nil, common.NewError("no url")
	}

	data := GetExternalLink(url)
	if len(data) == 0 {
		return nil, common.NewError("no result")
	}

	// if the data is a JSON object
	if strings.HasPrefix(data, "{") && strings.HasSuffix(data, "}") {
		var jsonData map[string]interface{}
		err = json.Unmarshal([]byte(data), &jsonData)
		if err != nil {
			logger.Warning("sub: Error unmarshalling JSON:", err)
			return nil, err
		}
		outbounds, ok := jsonData["outbounds"].([]any)
		if !ok {
			logger.Warning("sub: Error getting outbounds:", err)
			return nil, err
		}
		for _, outbound := range outbounds {
			outboundMap, ok := outbound.(map[string]interface{})
			if ok && len(outboundMap) > 0 {
				oType, _ := outboundMap["type"].(string)
				switch oType {
				case "urltest":
				case "direct":
				case "selector":
				case "block":
					continue
				default:
					result = append(result, outboundMap)
				}
			}
		}
		if len(result) == 0 {
			return nil, common.NewError("no result")
		}
		return result, nil
	} else {
		// if data is a text
		links := strings.Split(data, "\n")
		for _, link := range links {
			linkToJson, _, err := GetOutbound(link, 0)
			if err == nil {
				result = append(result, *linkToJson)
			}
		}
	}
	if len(result) == 0 {
		return nil, common.NewError("no result")
	}
	return result, nil
}
