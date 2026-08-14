package publish

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/pkg/snapshot"
)

// Publisher POSTs snapshots / catalog to the FireProxy server.
type Publisher struct {
	URL        string
	CatalogURL string
	AgentToken string
	Client     *http.Client
	mu         sync.Mutex
	lastErr    time.Time
}

func (p *Publisher) PostJSON(snap snapshot.Snapshot) error {
	return p.post(p.URL, snap)
}

func (p *Publisher) PostCatalog(cat inventory.Catalog) error {
	if p.CatalogURL == "" {
		return fmt.Errorf("catalog URL not configured")
	}
	return p.post(p.CatalogURL, cat)
}

func (p *Publisher) post(url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.AgentToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.AgentToken)
		req.Header.Set("X-API-Key", p.AgentToken)
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		p.logRare("post failed: %v", err)
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		err := fmt.Errorf("post status %d", res.StatusCode)
		p.logRare("%v", err)
		return err
	}
	return nil
}

func (p *Publisher) logRare(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if !p.lastErr.IsZero() && now.Sub(p.lastErr) < time.Minute {
		return
	}
	p.lastErr = now
	log.Printf("fireproxy-agent: "+format, args...)
}
