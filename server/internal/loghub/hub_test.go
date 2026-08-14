package loghub

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fireproxy/pkg/agentws"

	"github.com/gorilla/websocket"
)

func TestHubRefcountAndBroadcast(t *testing.T) {
	var starts, stops int
	h := &Hub{
		OnFirst: func() { starts++ },
		OnEmpty: func() { stops++ },
	}
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	c1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()
	time.Sleep(20 * time.Millisecond)
	if starts != 1 {
		t.Fatalf("starts %d", starts)
	}
	c2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if starts != 1 {
		t.Fatalf("starts should stay 1 got %d", starts)
	}

	h.Broadcast(&agentws.LogsBatch{
		ID:    "b1",
		Lines: []agentws.LogLine{{TS: 1, Source: "dnsmasq", Line: "x"}},
	})

	_ = c1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := c1.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) < 10 {
		t.Fatalf("msg %s", msg)
	}
	_ = c2.Close()
	_ = c1.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && stops < 1 {
		time.Sleep(10 * time.Millisecond)
	}
	if stops < 1 {
		t.Fatalf("stops %d", stops)
	}
}
