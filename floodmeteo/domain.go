package floodmeteo

import (
	"context"
	"fmt"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes floodmeteo as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/floodmeteo-cli/floodmeteo"
//
// exactly as a database/sql program enables a driver with
// `import _ "github.com/lib/pq"`. The init below registers it; the host then
// dereferences floodmeteo:// URIs by routing to the operations Register
// installs. The same Domain also builds the standalone floodmeteo binary (see
// cli.NewApp), so the binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the floodmeteo driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "floodmeteo",
		Hosts:  []string{"flood-api.open-meteo.com"},
		Identity: kit.Identity{
			Binary: "floodmeteo",
			Short:  "River discharge forecasts from the Open-Meteo Flood API.",
			Long: `River discharge forecasts from the Open-Meteo Flood API.

floodmeteo fetches daily river discharge data for any location on Earth via
the Open-Meteo Flood API. No API key required. Output is structured JSON/JSONL
that pipes into the rest of your tools.`,
			Site: "flood-api.open-meteo.com",
			Repo: "https://github.com/tamnd/floodmeteo-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// forecast: list daily river discharge for a location.
	kit.Handle(app, kit.OpMeta{
		Name:    "forecast",
		Group:   "read",
		List:    true,
		Summary: "Daily river discharge forecast for a location",
		URIType: "latlon",
		Args:    []kit.Arg{{Name: "ref", Help: "lat,lon pair or coordinate string", Optional: true}},
	}, listForecast)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	dcfg := DefaultConfig()
	if cfg.Rate > 0 {
		dcfg.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		dcfg.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		dcfg.Timeout = cfg.Timeout
	}
	c := NewClientFromConfig(dcfg)
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	return c, nil
}

// --- inputs ---

type forecastInput struct {
	Lat    float64 `kit:"flag" help:"latitude"`
	Lon    float64 `kit:"flag" help:"longitude"`
	Days   int     `kit:"flag" help:"number of forecast days (default 14)"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listForecast(ctx context.Context, in forecastInput, emit func(*DailyDischarge) error) error {
	days := in.Days
	if days <= 0 {
		days = 14
	}
	records, err := in.Client.Forecast(ctx, in.Lat, in.Lon, days)
	if err != nil {
		return mapErr(err)
	}
	for _, r := range records {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: URI-native string functions, pure and network-free ---

// Classify turns any accepted input into the canonical (type, id).
// A lat,lon pair is classified as "latlon"; anything else as "query".
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if isLatLon(input) {
		return "latlon", input, nil
	}
	return "query", input, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "latlon":
		lat, lon, err := parseLatLon(id)
		if err != nil {
			return "", errs.Usage("invalid latlon %q: %v", id, err)
		}
		return fmt.Sprintf("https://flood-api.open-meteo.com/v1/flood?latitude=%g&longitude=%g", lat, lon), nil
	case "query":
		return "https://flood-api.open-meteo.com/v1/flood?q=" + id, nil
	default:
		return "", errs.Usage("floodmeteo has no resource type %q", uriType)
	}
}

// --- helpers ---

func isLatLon(s string) bool {
	_, _, err := parseLatLon(s)
	return err == nil
}

func parseLatLon(s string) (lat, lon float64, err error) {
	_, err = fmt.Sscanf(strings.ReplaceAll(s, ",", " "), "%f %f", &lat, &lon)
	if err != nil {
		return 0, 0, err
	}
	return lat, lon, nil
}

func mapErr(err error) error {
	return err
}
