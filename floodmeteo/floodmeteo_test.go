package floodmeteo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/floodmeteo-cli/floodmeteo"
)

func floodResponse(times []string, discharge []float64) []byte {
	type daily struct {
		Time           []string  `json:"time"`
		RiverDischarge []float64 `json:"river_discharge"`
	}
	type resp struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Daily     daily   `json:"daily"`
	}
	b, _ := json.Marshal(resp{
		Latitude:  52.52,
		Longitude: 13.41,
		Daily:     daily{Time: times, RiverDischarge: discharge},
	})
	return b
}

func newTestClient(baseURL string) *floodmeteo.Client {
	cfg := floodmeteo.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0
	cfg.Retries = 3
	cfg.Timeout = 5 * time.Second
	return floodmeteo.NewClientFromConfig(cfg)
}

func TestForecast(t *testing.T) {
	times := []string{"2026-06-14", "2026-06-15", "2026-06-16"}
	discharge := []float64{0.63, 1.06, 1.03}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(floodResponse(times, discharge))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	records, err := c.Forecast(context.Background(), 52.52, 13.41, 3)
	if err != nil {
		t.Fatalf("Forecast: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}
	for i, r := range records {
		if r.Time != times[i] {
			t.Errorf("record[%d].Time = %q, want %q", i, r.Time, times[i])
		}
		if r.RiverDischarge != discharge[i] {
			t.Errorf("record[%d].RiverDischarge = %v, want %v", i, r.RiverDischarge, discharge[i])
		}
	}
}

func TestForecastRetriesOn503(t *testing.T) {
	times := []string{"2026-06-14"}
	discharge := []float64{1.5}
	var hits int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(floodResponse(times, discharge))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	start := time.Now()
	records, err := c.Forecast(context.Background(), 52.52, 13.41, 1)
	if err != nil {
		t.Fatalf("Forecast after retry: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestForecastHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Forecast(context.Background(), 0, 0, 1)
	if err == nil {
		t.Error("expected error on 400, got nil")
	}
}
