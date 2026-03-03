package gtfs

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/jamespfennell/gtfs"

	"github.com/teclara/sixrail/api/internal/models"
)

type StaticStore struct {
	mu     sync.RWMutex
	stops  []models.Stop
	routes map[string]models.Route
}

func NewStaticStore(zipData []byte) (*StaticStore, error) {
	s := &StaticStore{}
	if err := s.load(zipData); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *StaticStore) load(zipData []byte) error {
	static, err := gtfs.ParseStatic(zipData, gtfs.ParseStaticOptions{})
	if err != nil {
		return fmt.Errorf("parsing GTFS static: %w", err)
	}

	stops := make([]models.Stop, 0, len(static.Stops))
	for i := range static.Stops {
		gs := &static.Stops[i]
		if gs.Latitude == nil || gs.Longitude == nil {
			continue
		}
		var parentID string
		if gs.Parent != nil {
			parentID = gs.Parent.Id
		}
		stops = append(stops, models.Stop{
			ID:       gs.Id,
			Code:     gs.Code,
			Name:     gs.Name,
			Lat:      *gs.Latitude,
			Lon:      *gs.Longitude,
			ParentID: parentID,
		})
	}

	routes := make(map[string]models.Route, len(static.Routes))
	for i := range static.Routes {
		gr := &static.Routes[i]
		routes[gr.Id] = models.Route{
			ID:        gr.Id,
			ShortName: gr.ShortName,
			LongName:  gr.LongName,
			Color:     gr.Color,
			TextColor: gr.TextColor,
			Type:      int(gr.Type),
		}
	}

	s.mu.Lock()
	s.stops = stops
	s.routes = routes
	s.mu.Unlock()

	slog.Info("GTFS static loaded", "stops", len(stops), "routes", len(routes))
	return nil
}

func (s *StaticStore) Refresh(zipData []byte) error {
	return s.load(zipData)
}

func (s *StaticStore) AllStops() []models.Stop {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Stop, len(s.stops))
	copy(out, s.stops)
	return out
}

func (s *StaticStore) GetRoute(id string) (models.Route, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.routes[id]
	return r, ok
}
