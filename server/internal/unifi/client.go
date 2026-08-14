package unifi

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func defaultHTTP() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // local UniFi OS self-signed
		},
	}
}

func getJSON(httpClient *http.Client, base, apiKey, path string) ([]byte, error) {
	u := strings.TrimRight(base, "/") + path
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-KEY", apiKey)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("HTTP %d %s", res.StatusCode, msg)
	}
	return body, nil
}

func putJSON(httpClient *http.Client, base, apiKey, path string, payload any) ([]byte, int, error) {
	u := strings.TrimRight(base, "/") + path
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-KEY", apiKey)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, res.StatusCode, err
	}
	if res.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return body, res.StatusCode, fmt.Errorf("HTTP %d %s", res.StatusCode, msg)
	}
	return body, res.StatusCode, nil
}
