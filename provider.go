package sidecar

import (
	"fmt"
	"net/http"
)

// DemoProvider makes the API runnable before the real SiriusXM client is connected.
// Replace this with your actual SiriusXM implementation.
type DemoProvider struct {
	stations []Station
}

func NewDemoProvider() *DemoProvider {
	return &DemoProvider{stations: []Station{
		{ID: "octane", Name: "Octane"},
		{ID: "siriusxmu", Name: "SiriusXM U"},
	}}
}

func (p *DemoProvider) Stations() []Station { return p.stations }

func (p *DemoProvider) NowPlaying(id string) (NowPlaying, error) {
	for _, s := range p.stations {
		if s.ID == id {
			return NowPlaying{Title: "Not connected", Artist: "", Album: ""}, nil
		}
	}
	return NowPlaying{}, fmt.Errorf("station %q not found", id)
}

func (p *DemoProvider) Stream(id string, w http.ResponseWriter, r *http.Request) error {
	for _, s := range p.stations {
		if s.ID == id {
			return streamPlaceholder(w)
		}
	}
	return fmt.Errorf("station %q not found", id)
}

func streamPlaceholder(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}
