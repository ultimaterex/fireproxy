package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"fireproxy/agent/internal/agentevents"
	"fireproxy/agent/internal/collect"
	"fireproxy/agent/internal/config"
	"fireproxy/agent/internal/publish"
	"fireproxy/agent/internal/svclogs"
	"fireproxy/agent/internal/update"
	"fireproxy/agent/internal/wsclient"
	"fireproxy/pkg/agentws"
)

// Version is set via -ldflags -X main.Version=...
var Version = "0.0.0"

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg.Version = Version

	gr := collect.NewGoRedis(cfg.RedisAddr)
	defer gr.Close()

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	col := &collect.Collector{
		Hostname:      cfg.Hostname,
		Ifaces:        cfg.Ifaces,
		SysfsRoot:     cfg.SysfsRoot,
		ProcLoadPath:  cfg.ProcLoadPath,
		CPUOnlinePath: cfg.CPUOnlinePath,
		ProcStatPath:  cfg.ProcStatPath,
		CollectCPU:    cfg.CollectCPU,
		Redis:         gr,
		ACL:           &collect.ACLTracker{Path: cfg.ACLLogPath},
		FwapcURL:      cfg.FwapcURL,
		FireRouterURL: cfg.FireRouterURL,
		HTTPGet: func(url string) ([]byte, error) {
			resp, err := httpClient.Get(url)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		},
	}

	pub := &publish.Publisher{
		URL:        cfg.PushURL,
		CatalogURL: cfg.CatalogURL,
		AgentToken: cfg.AgentToken,
		Client:     httpClient,
	}
	catalogPub := &publish.Publisher{
		URL:        cfg.PushURL,
		CatalogURL: cfg.CatalogURL,
		AgentToken: cfg.AgentToken,
		Client:     &http.Client{Timeout: 30 * time.Second},
	}

	cursorStore, err := svclogs.LoadCursorStore(cfg.LogCursorPath)
	if err != nil {
		log.Fatalf("log cursors: %v", err)
	}
	aclReader := &svclogs.ACLFileReader{Path: cfg.ACLLogPath}
	logCol := &svclogs.Collector{
		Store: cursorStore,
		Journal: &svclogs.JournalReader{
			Run: wsclient.JournalCtlRunner,
		},
		ACL: aclReader,
	}
	logFollow := &svclogs.Follow{
		Store: cursorStore,
		ACL:   aclReader,
	}

	var catalogMu sync.Mutex
	catalogBusy := false
	tickCatalog := func() error {
		catalogMu.Lock()
		if catalogBusy {
			catalogMu.Unlock()
			log.Printf("fireproxy-agent: catalog already running")
			return fmt.Errorf("busy")
		}
		catalogBusy = true
		catalogMu.Unlock()
		defer func() {
			catalogMu.Lock()
			catalogBusy = false
			catalogMu.Unlock()
		}()

		cat, err := col.CollectCatalog(time.Now())
		if err != nil {
			log.Printf("fireproxy-agent: catalog: %v", err)
			return err
		}
		if err := catalogPub.PostCatalog(cat); err != nil {
			return err
		}
		log.Printf("fireproxy-agent: posted catalog devices=%d network=%d policies=%d tags=%d",
			len(cat.Devices), len(cat.Network), len(cat.Policies), len(cat.Tags))
		return nil
	}

	eventStore := &agentevents.Store{Path: filepath.Join(filepath.Dir(cfg.LogCursorPath), "agent-events.jsonl")}

	ws := &wsclient.Client{
		URL:        cfg.WSURL,
		AgentToken: cfg.AgentToken,
		Host:       cfg.Hostname,
		Version:    cfg.Version,
		SelfUpdate: cfg.SelfUpdate,
		Collector:  logCol,
		Follow:     logFollow,
		Events:     eventStore,
		OnCatalog:  tickCatalog,
		OnUpdate: func(u *agentws.AgentUpdate) {
			if !update.Allowed(cfg.SelfUpdate, u) {
				return
			}
			log.Printf("fireproxy-agent: applying update %s", u.Version)
			dest := update.DestFromEnv()
			if err := update.Apply(u, dest); err != nil {
				log.Printf("fireproxy-agent: update failed: %v", err)
				_ = eventStore.Append("update_failed", err.Error())
			}
		},
	}

	log.Printf("fireproxy-agent starting host=%s version=%s self_update=%v interval=%s inventory=%s push=%s catalog=%s ws=%s",
		cfg.Hostname, cfg.Version, cfg.SelfUpdate, cfg.Interval, cfg.InventoryInterval, cfg.PushURL, cfg.CatalogURL, cfg.WSURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ws.Run(ctx)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	tickMetrics := func() {
		snap, err := col.Collect(time.Now())
		if err != nil {
			log.Printf("fireproxy-agent: collect: %v", err)
			return
		}
		if err := pub.PostJSON(snap); err != nil {
			return
		}
		log.Printf("fireproxy-agent: posted metrics ts=%d ifaces=%d dns_restarts=%d",
			snap.TS, len(snap.Ifaces), snap.DNSRestarts)
	}

	tickMetrics()
	tickCatalog()

	metricsTicker := time.NewTicker(cfg.Interval)
	invTicker := time.NewTicker(cfg.InventoryInterval)
	defer metricsTicker.Stop()
	defer invTicker.Stop()

	logStagger := time.NewTimer(cfg.LogSyncStagger)
	defer logStagger.Stop()
	var logTicker *time.Ticker
	var logCh <-chan time.Time

	for {
		select {
		case <-stop:
			log.Printf("fireproxy-agent stopping")
			cancel()
			return
		case <-metricsTicker.C:
			tickMetrics()
		case <-invTicker.C:
			tickCatalog()
		case <-logStagger.C:
			ws.HourlySync()
			logTicker = time.NewTicker(cfg.LogSyncInterval)
			defer logTicker.Stop()
			logCh = logTicker.C
		case <-logCh:
			ws.HourlySync()
		}
	}
}
