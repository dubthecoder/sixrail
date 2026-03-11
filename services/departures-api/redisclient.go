package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/teclara/railsix/shared/cache"
	"github.com/teclara/railsix/shared/gtfsrt"
	"github.com/teclara/railsix/shared/models"
)

// Redis key constants matching the realtime-poller's storage layout.
const (
	keyAlerts          = "transit:alerts"
	keyTripUpdates     = "transit:trip-updates"
	keyServiceGlance   = "transit:service-glance"
	keyExceptions      = "transit:exceptions"
	keyUnionDepartures = "transit:union-departures"
)

// RedisClient wraps Redis reads needed by departure logic.
type RedisClient struct {
	rc *redis.Client
}

// NewRedisClient creates a RedisClient.
func NewRedisClient(rc *redis.Client) *RedisClient {
	return &RedisClient{rc: rc}
}

// GetTripUpdate retrieves a trip update from the Redis hash by trip ID.
func (r *RedisClient) GetTripUpdate(ctx context.Context, tripID string) (gtfsrt.RawTripUpdate, bool) {
	var update gtfsrt.RawTripUpdate
	err := cache.GetHashFieldJSON(ctx, r.rc, keyTripUpdates, tripID, &update)
	if err != nil {
		return gtfsrt.RawTripUpdate{}, false
	}
	return update, true
}

// GetServiceGlanceEntry retrieves a service glance entry by trip number.
func (r *RedisClient) GetServiceGlanceEntry(ctx context.Context, tripNum string) (models.ServiceGlanceEntry, bool) {
	var entry models.ServiceGlanceEntry
	err := cache.GetHashFieldJSON(ctx, r.rc, keyServiceGlance, tripNum, &entry)
	if err != nil {
		return models.ServiceGlanceEntry{}, false
	}
	return entry, true
}

// IsTripCancelled checks if a trip number is in the exceptions set.
func (r *RedisClient) IsTripCancelled(ctx context.Context, tripNum string) bool {
	ok, err := cache.IsMember(ctx, r.rc, keyExceptions, tripNum)
	if err != nil {
		return false
	}
	return ok
}

// GetUnionDepartureByTrip finds a Union departure by trip number.
func (r *RedisClient) GetUnionDepartureByTrip(ctx context.Context, tripNum string) (models.UnionDeparture, bool) {
	deps := r.GetUnionDepartures(ctx)
	for _, d := range deps {
		if d.TripNumber == tripNum {
			return d, true
		}
	}
	return models.UnionDeparture{}, false
}

// GetUnionDepartures returns all cached Union Station departures.
func (r *RedisClient) GetUnionDepartures(ctx context.Context) []models.UnionDeparture {
	var deps []models.UnionDeparture
	if err := cache.GetJSON(ctx, r.rc, keyUnionDepartures, &deps); err != nil {
		return nil
	}
	return deps
}

// GetAlerts returns all cached alerts.
func (r *RedisClient) GetAlerts(ctx context.Context) []models.Alert {
	var alerts []models.Alert
	if err := cache.GetJSON(ctx, r.rc, keyAlerts, &alerts); err != nil {
		return nil
	}
	return alerts
}

// GetAllServiceGlanceMap returns all service glance entries indexed by trip number.
func (r *RedisClient) GetAllServiceGlanceMap(ctx context.Context) map[string]models.ServiceGlanceEntry {
	all, err := cache.GetHashAllJSON[models.ServiceGlanceEntry](ctx, r.rc, keyServiceGlance)
	if err != nil {
		return nil
	}
	return all
}

// GetAllServiceGlance returns all service glance entries.
func (r *RedisClient) GetAllServiceGlance(ctx context.Context) []models.ServiceGlanceEntry {
	all, err := cache.GetHashAllJSON[models.ServiceGlanceEntry](ctx, r.rc, keyServiceGlance)
	if err != nil {
		return nil
	}
	entries := make([]models.ServiceGlanceEntry, 0, len(all))
	for _, e := range all {
		entries = append(entries, e)
	}
	return entries
}

// GetNextService retrieves cached NextService data for a stop code.
func (r *RedisClient) GetNextService(ctx context.Context, stopCode string) ([]models.NextServiceLine, bool) {
	key := "transit:next-service:" + stopCode
	var lines []models.NextServiceLine
	if err := cache.GetJSON(ctx, r.rc, key, &lines); err != nil {
		return nil, false
	}
	return lines, true
}

// SetNextService caches NextService data for a stop code with 30s TTL.
func (r *RedisClient) SetNextService(ctx context.Context, stopCode string, lines []models.NextServiceLine) {
	key := "transit:next-service:" + stopCode
	_ = cache.SetJSON(ctx, r.rc, key, lines, 30*time.Second)
}

// GetFares retrieves cached fare info between two stations.
func (r *RedisClient) GetFares(ctx context.Context, from, to string) ([]models.FareInfo, bool) {
	key := "transit:fares:" + from + ":" + to
	var fares []models.FareInfo
	if err := cache.GetJSON(ctx, r.rc, key, &fares); err != nil {
		return nil, false
	}
	return fares, true
}

// SetFares caches fare info between two stations with 1h TTL.
func (r *RedisClient) SetFares(ctx context.Context, from, to string, fares []models.FareInfo) {
	key := "transit:fares:" + from + ":" + to
	_ = cache.SetJSON(ctx, r.rc, key, fares, time.Hour)
}
