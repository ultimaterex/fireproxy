package collect

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCatalogAssets(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r := &memRedis{
			hgetall: map[string]map[string]string{
				"sys:network:info": {
					"br0":  `{"name":"br0","desc":"Core","type":"lan","uuid":"u1","ip_address":"203.0.113.1","subnet":"203.0.113.1/24"}`,
					"eth1": `{"name":"eth1","desc":"ISP A","type":"wan","uuid":"w1"}`,
				},
				"event:state:cache": {
					"dualwan_state:primary_standby": `{"labels":{"wanType":"primary_standby"}}`,
				},
			},
		}
		var gotURLs []string
		httpGet := func(url string) ([]byte, error) {
			gotURLs = append(gotURLs, url)
			switch {
			case strings.HasSuffix(url, "/status/switch"):
				return []byte(fwapcSwitchFixture), nil
			case strings.HasSuffix(url, "/status/wired_station"):
				return []byte(wiredTreeFixture), nil
			case strings.HasSuffix(url, "/config/lans"):
				return []byte(frLansFixture), nil
			case strings.HasSuffix(url, "/config/wans"):
				return []byte(frWansFixture), nil
			case strings.HasSuffix(url, "/config/interfaces"), strings.HasSuffix(url, "/config/active"):
				return []byte(`{}`), nil
			default:
				t.Fatalf("unexpected url: %s", url)
				return nil, errors.New("unexpected")
			}
		}
		c := &Collector{
			Hostname:      "Firewalla",
			Redis:         r,
			FwapcURL:      "http://127.0.0.1:8841/v1",
			FireRouterURL: "http://127.0.0.1:8837/v1",
			HTTPGet:       httpGet,
		}
		cat, err := c.CollectCatalog(time.Unix(100, 0))
		if err != nil {
			t.Fatal(err)
		}
		if cat.Box == nil || cat.Box.WanType != "failover" {
			t.Fatalf("wan_type: %+v", cat.Box)
		}
		if len(cat.Switches) != 1 {
			t.Fatalf("switches: %+v", cat.Switches)
		}
		sw := cat.Switches[0]
		if sw.Name != "Switch-1" || sw.UplinkPort != "1" || sw.ParentNIC != "eth3" {
			t.Fatalf("switch: %+v", sw)
		}
		if len(cat.Topo) != 1 || cat.Topo[0].Type != "box" {
			t.Fatalf("topo root: %+v", cat.Topo)
		}
		if len(cat.Topo[0].Children) != 1 {
			t.Fatalf("topo children: %+v", cat.Topo[0].Children)
		}
		swNode := cat.Topo[0].Children[0]
		if swNode.Type != "switch" || swNode.MAC != "02:00:00:00:00:10" {
			t.Fatalf("switch node: %+v", swNode)
		}
		if len(swNode.Children) != 0 {
			t.Fatalf("device nodes must not appear in tree: %+v", swNode.Children)
		}
		if len(swNode.Clients) != 2 {
			t.Fatalf("clients: %v", swNode.Clients)
		}
		var br0 *struct {
			intfs []string
		}
		for i := range cat.Network {
			if cat.Network[i].Name == "br0" {
				br0 = &struct{ intfs []string }{intfs: cat.Network[i].Intfs}
				break
			}
		}
		if br0 == nil {
			t.Fatalf("network: %+v", cat.Network)
		}
		if len(br0.intfs) != 2 || br0.intfs[0] != "eth0" || br0.intfs[1] != "eth3" {
			t.Fatalf("br0 intfs: %v", br0.intfs)
		}
		if len(gotURLs) != 6 {
			t.Fatalf("urls: %v", gotURLs)
		}
	})

	t.Run("HTTPGet error", func(t *testing.T) {
		r := &memRedis{
			hgetall: map[string]map[string]string{
				"sys:network:info": {
					"br0": `{"name":"br0","desc":"Core","type":"lan","uuid":"u1","subnet":"203.0.113.1/24"}`,
				},
			},
		}
		c := &Collector{
			Hostname: "Firewalla",
			Redis:    r,
			HTTPGet: func(string) ([]byte, error) {
				return nil, errors.New("offline")
			},
		}
		cat, err := c.CollectCatalog(time.Unix(100, 0))
		if err != nil {
			t.Fatal(err)
		}
		if len(cat.Switches) != 0 {
			t.Fatalf("switches should be empty: %+v", cat.Switches)
		}
	})
}
