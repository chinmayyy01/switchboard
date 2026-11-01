package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Geo struct {
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Display string  `json:"display"`
}

var (
	geoMu    sync.Mutex
	geoCache = map[string]Geo{}
	geoHTTP  = &http.Client{Timeout: 6 * time.Second}
)

func Geocode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(q) < 4 {
			http.Error(w, "q too short", http.StatusBadRequest)
			return
		}
		geoMu.Lock()
		g, ok := geoCache[q]
		geoMu.Unlock()
		if !ok {
			var err error
			g, err = lookupGeo(q)
			if err != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			geoMu.Lock()
			geoCache[q] = g
			geoMu.Unlock()
		}
		writeJSON(w, g)
	}
}

func lookupGeo(q string) (Geo, error) {
	u := "https://nominatim.openstreetmap.org/search?format=json&limit=1&q=" + url.QueryEscape(q)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return Geo{}, err
	}
	req.Header.Set("User-Agent", "switchboard-demo/1.0")
	resp, err := geoHTTP.Do(req)
	if err != nil {
		return Geo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Geo{}, fmt.Errorf("geocode: %s", resp.Status)
	}
	var out []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Geo{}, err
	}
	if len(out) == 0 {
		return Geo{}, fmt.Errorf("no match")
	}
	lat, err := strconv.ParseFloat(out[0].Lat, 64)
	if err != nil {
		return Geo{}, err
	}
	lon, err := strconv.ParseFloat(out[0].Lon, 64)
	if err != nil {
		return Geo{}, err
	}
	return Geo{Lat: lat, Lon: lon, Display: out[0].DisplayName}, nil
}
