package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"fireproxy/pkg/agentws"
	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/agenthub"
	"fireproxy/server/internal/agentpkg"
	"fireproxy/server/internal/api"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/config"
	"fireproxy/server/internal/controlhist"
	"fireproxy/server/internal/enroll"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/geo"
	"fireproxy/server/internal/ingest"
	"fireproxy/server/internal/loghub"
	"fireproxy/server/internal/modules"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
	"fireproxy/server/internal/unifi"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "0.0.0"

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	authCfg, err := auth.LoadFromEnv()
	if err != nil {
		log.Fatalf("auth config: %v", err)
	}
	if authCfg.Disabled {
		log.Printf("WARNING: AUTH_DISABLED=true — authentication is OFF (non-production only)")
	}
	if authCfg.Disabled && authCfg.DevAgentToken != "" {
		log.Printf("WARNING: AUTH_DEV_AGENT_TOKEN / dev agent auth is active (non-production)")
	}

	mem := store.NewMemoryStore(cfg.StoreSize)
	catalog := store.NewCatalogStore()
	if !authCfg.Disabled && cfg.DataPath == "" {
		log.Fatalf("auth enabled but DATA_PATH is empty — sessions/keys/agent credentials require persist")
	}
	if cfg.DataPath != "" {
		p, err := store.OpenPersist(cfg.DataPath, cfg.RetentionDays)
		if err != nil {
			if !authCfg.Disabled {
				log.Fatalf("auth enabled but persist open failed: %v", err)
			}
			log.Printf("persist disabled: %v", err)
		} else {
			defer p.Close()
			mem.AttachPersist(p)
			catalog.AttachPersist(p)
			n, _ := p.Count()
			log.Printf("persist %s retention=%dd points=%d", p.Path(), p.RetentionDays(), n)
			if !authCfg.Disabled {
				if err := auth.SyncPasswordFingerprint(p, authCfg.Password); err != nil {
					log.Fatalf("auth password fingerprint: %v", err)
				}
			}
		}
	}

	ns := &unifi.PrefsStore{}
	tpPrefs := &tplink.PrefsStore{}
	if db := mem.Persist(); db != nil {
		if raw, ok, err := db.LoadModulePrefs(); err != nil {
			log.Printf("persist module prefs: %v", err)
		} else if ok {
			ns.Set(unifi.PrefsFromMap(raw))
			tpPrefs.Set(tplink.PrefsFromMap(raw))
		}
		savePrefs := func(patch map[string]any) {
			raw, _, err := db.LoadModulePrefs()
			if err != nil {
				log.Printf("persist module prefs: %v", err)
				return
			}
			if raw == nil {
				raw = map[string]any{}
			}
			for k, v := range patch {
				raw[k] = v
			}
			if err := db.SaveModulePrefs(raw); err != nil {
				log.Printf("persist module prefs: %v", err)
			}
		}
		ns.SetPersist(func(prefs unifi.Prefs) {
			savePrefs(map[string]any{"unifi-sync": prefs})
		})
		tpPrefs.SetPersist(func(prefs tplink.Prefs) {
			savePrefs(map[string]any{"tplink-sync": prefs})
		})
	}

	facts := modules.DefaultFactories()
	var reg *modules.Registry
	var tplinkStore *tplink.Store
	var fwAppSvc *fwapp.Service
	if db := mem.Persist(); db != nil {
		tplinkStore = tplink.NewStore(db, os.Getenv("FIREPROXY_SECRETS_KEY"))
		fwAppSvc = fwapp.NewService(db, os.Getenv("FIREPROXY_SECRETS_KEY"))
	} else {
		tplinkStore = tplink.NewStore(nil, os.Getenv("FIREPROXY_SECRETS_KEY"))
		fwAppSvc = fwapp.NewService(fwapp.NewMemStore(), os.Getenv("FIREPROXY_SECRETS_KEY"))
	}
	facts["tplink-sync"] = func() modules.Module {
		return tplink.New(tplink.ModuleConfig{
			Store:    tplinkStore,
			Interval: time.Duration(tplink.DefaultPollSec) * time.Second,
			PollSec:  tpPrefs.PollSec,
		})
	}
	facts["fw-app"] = func() modules.Module {
		return &fwapp.Module{Svc: fwAppSvc}
	}
	fwAppSvc.IndexSpeedtest = func(results []fwapp.SpeedtestResult) {
		by := map[string][]inventory.SpeedtestPoint{}
		for _, r := range results {
			uuid := strings.TrimSpace(r.WanUUID)
			if uuid == "" || (r.Down == 0 && r.Up == 0) {
				continue
			}
			by[uuid] = append(by[uuid], inventory.SpeedtestPoint{
				TS: r.TS, Down: r.Down, Up: r.Up, Ping: r.Ping,
				ServerID: r.ServerID, Server: r.Server, Location: r.Location,
			})
		}
		for uuid, pts := range by {
			catalog.MergeSpeedtest(pts, uuid)
		}
	}
	facts["unifi-sync"] = func() modules.Module {
		return unifi.New(unifi.Config{
			BaseURL: cfg.UnifiBaseURL,
			APIKey:  cfg.UnifiAPIKey,
			AfterOK: func() {
				m, ok := reg.Running("unifi-sync").(*unifi.Module)
				if !ok || m == nil {
					return
				}
				fetch := func() ([]unifi.User, []unifi.FWHost, []string, error) {
					users, err := m.FetchUsers()
					if err != nil {
						return nil, nil, nil, err
					}
					cat, ok := catalog.Get()
					if !ok {
						return nil, nil, nil, errors.New("no catalog")
					}
					return users, unifi.HostsFromCatalog(inventory.ActiveDevices(cat.Devices), m.ClientIPs()), m.HardwareMACs(), nil
				}
				unifi.AutoFillEmpty(ns, fetch, m.ApplyRows)
				in := unifi.ReportInput{}
				if cat, ok := catalog.Get(); ok {
					hw := m.HardwareMACs()
					fwDevs := inventory.ActiveDevices(cat.Devices)
					in.VLAN = unifi.VLANMismatch(unifi.VLANInput{
						Devices:  fwDevs,
						Network:  cat.Network,
						Stations: m.Stations(),
						Hardware: hw,
						SXVIDs:   unifi.ClientVLANsFromSwitches(cat.Switches),
					})
					if snap, ok := m.Snapshot(); ok {
						switches, _ := unifi.Merge(cat, snap)
						in.STP = unifi.STPFindings(switches, snap.STP)
						in.Offline = unifi.OfflineUniFi(switches)
					}
					in.Unknown = unifi.UnknownMAC(unifi.UnknownInput{
						Devices:  fwDevs,
						Stations: m.Stations(),
						ClientIP: m.ClientIPs(),
						Hardware: hw,
					})
					in.Pending = m.Pending()
				}
				unifi.RefreshAudit(ns, fetch, unifi.BuildReport(in))
			},
		})
	}
	reg = modules.NewRegistry(cfg.ModuleEnable, facts)
	if p := mem.Persist(); p != nil {
		saved, ok, err := p.LoadModules()
		if err != nil {
			log.Printf("persist modules: %v", err)
		} else if ok {
			reg.Apply(saved)
		}
		reg.SetPersist(func(enable map[string]bool) {
			if err := p.SaveModules(enable); err != nil {
				log.Printf("persist modules: %v", err)
			}
		})
	}
	reg.StartAll()
	defer reg.StopAll()

	mux := http.NewServeMux()
	mmdbPath := geo.ResolvePath(cfg.GeoIPMMDB, cfg.DataPath)
	auto := cfg.MaxMindLicenseKey != ""
	holder := geo.NewHolder(geo.OpenMMDB(mmdbPath))
	geoCtx, geoCancel := context.WithCancel(context.Background())
	defer geoCancel()
	if auto {
		up := &geo.Updater{
			AccountID:  cfg.MaxMindAccountID,
			LicenseKey: cfg.MaxMindLicenseKey,
			Path:       mmdbPath,
			Interval:   cfg.GeoIPUpdateEvery,
			Holder:     holder,
		}
		if err := up.Once(geoCtx); err != nil {
			log.Printf("geo: %v", err)
		}
		go up.Loop(geoCtx)
	}
	controlHist := controlhist.New(mem.Persist())
	apiServer := &api.Server{
		Store:             mem,
		CatalogStore:      catalog,
		Modules:           reg,
		Geo:               holder,
		NameSync:          ns,
		TPLinkPrefs:       tpPrefs,
		Persist:           mem.Persist(),
		ControlHist:       controlHist,
		AuthDisabled:      authCfg.Disabled,
		TPLink:            tplinkStore,
		FWApp:             fwAppSvc,
		Enroll:            &enroll.CodeStore{},
		InstallScriptPath: cfg.InstallScriptPath,
	}
	pkg, err := agentpkg.Open(cfg.AgentBinDir)
	if err != nil {
		log.Printf("agent package: %v", err)
	} else {
		apiServer.AgentPackage = pkg
		if pkg.Missing() {
			log.Printf("agent package missing under %s", cfg.AgentBinDir)
		} else {
			log.Printf("agent package version=%s", pkg.Version)
		}
	}
	agentAuth := &auth.AgentCreds{Persist: mem.Persist(), Cfg: authCfg}
	hub := &agenthub.Hub{
		Auth:         agentAuth,
		Persist:      mem.Persist(),
		Package:      pkg,
		PublicOrigin: authCfg.PublicOrigin,
		PublicBase: func() string {
			if p := mem.Persist(); p != nil {
				if u := p.PublicBaseURL(); u != "" {
					return u
				}
			}
			return ""
		},
	}
	uiLogs := &loghub.Hub{PublicOrigin: authCfg.PublicOrigin}
	uiLogs.OnFirst = func() {
		go func() {
			// Let the browser attach onmessage before status fan-out.
			time.Sleep(50 * time.Millisecond)
			if err := hub.StartLive(); err != nil {
				uiLogs.BroadcastStatus(&agentws.LogsStatus{Error: "agent_offline", Live: false})
				return
			}
			uiLogs.BroadcastStatus(&agentws.LogsStatus{Live: true})
		}()
	}
	uiLogs.OnEmpty = func() {
		_ = hub.StopLive()
	}
	hub.UI = uiLogs
	apiServer.AgentHub = hub
	apiServer.LogHub = uiLogs
	apiServer.Routes(mux)
	mux.HandleFunc("GET /healthz", auth.Healthz)
	(&auth.Handlers{Persist: mem.Persist(), Cfg: authCfg}).Register(mux)
	mux.Handle("/v1/ingest", &ingest.Handler{Store: mem, Auth: agentAuth})
	mux.Handle("POST /v1/catalog", &ingest.CatalogHandler{Store: catalog, Auth: agentAuth})
	mux.Handle("POST /v1/devices", &ingest.DevicesHandler{Store: catalog, Auth: agentAuth})

	gate := &auth.Middleware{
		Cfg:     authCfg,
		Persist: mem.Persist(),
		Agents:  agentAuth,
		Limiter: auth.NewIPRateLimiter(10, time.Minute),
	}
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: gate.Handler(mux)}
	go func() {
		log.Printf("fireproxy-server %s listening on %s", Version, cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Printf("fireproxy-server stopping")
	_ = srv.Close()
}
