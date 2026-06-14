// Package floodmeteo is the library behind the floodmeteo command line:
// the HTTP client, request shaping, and the typed data models for the
// Open-Meteo Flood API (river discharge forecasts).
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package floodmeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultUserAgent identifies the client to Open-Meteo.
const DefaultUserAgent = "floodmeteo/dev (+https://github.com/tamnd/floodmeteo-cli)"

// Config holds the tunable parameters for a Client.
type Config struct {
	BaseURL string
	Rate    time.Duration
	Retries int
	Timeout time.Duration
}

// DefaultConfig returns the production config.
func DefaultConfig() Config {
	return Config{
		BaseURL: "https://flood-api.open-meteo.com",
		Rate:    0,
		Retries: 3,
		Timeout: 15 * time.Second,
	}
}

// Client talks to the Open-Meteo Flood API over HTTPS.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	cfg       Config

	last time.Time
}

// NewClient returns a Client with DefaultConfig settings.
func NewClient() *Client {
	cfg := DefaultConfig()
	return &Client{
		HTTP:      &http.Client{Timeout: cfg.Timeout},
		UserAgent: DefaultUserAgent,
		cfg:       cfg,
	}
}

// NewClientFromConfig builds a Client from the provided Config.
func NewClientFromConfig(cfg Config) *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: cfg.Timeout},
		UserAgent: DefaultUserAgent,
		cfg:       cfg,
	}
}

// DailyDischarge is one day of river discharge forecast data.
type DailyDischarge struct {
	Time           string  `json:"time" kit:"id"`
	RiverDischarge float64 `json:"river_discharge"`
}

// wireFloodResponse is the raw JSON shape returned by the Flood API.
type wireFloodResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Daily     struct {
		Time           []string  `json:"time"`
		RiverDischarge []float64 `json:"river_discharge"`
	} `json:"daily"`
}

// Forecast fetches the daily river discharge forecast for the given coordinates
// and number of forecast days.
func (c *Client) Forecast(ctx context.Context, lat, lon float64, days int) ([]*DailyDischarge, error) {
	url := fmt.Sprintf("%s/v1/flood?latitude=%g&longitude=%g&daily=river_discharge&forecast_days=%d",
		c.cfg.BaseURL, lat, lon, days)
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var wire wireFloodResponse
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode flood response: %w", err)
	}
	n := len(wire.Daily.Time)
	if len(wire.Daily.RiverDischarge) < n {
		n = len(wire.Daily.RiverDischarge)
	}
	out := make([]*DailyDischarge, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, &DailyDischarge{
			Time:           wire.Daily.Time[i],
			RiverDischarge: wire.Daily.RiverDischarge[i],
		})
	}
	return out, nil
}

// Get fetches url and returns the response body. It paces and retries according
// to the client's settings.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
