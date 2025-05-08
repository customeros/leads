package nats_internal

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/customeros/leads/internal/telemetry"
)

// SessionCache provides visitor lookup caching using NATS KV
type SessionCache struct {
	kv nats.KeyValue
}

const SESSION_CACHE_BUCKET = "session_cache"

// NewIPCache creates a new NATS KV-backed session by visitor cache
func NewSessionCache(js nats.JetStreamContext) (*SessionCache, error) {
	// First try to get the bucket
	kv, err := js.KeyValue(SESSION_CACHE_BUCKET)
	// If bucket doesn't exist, create it
	if err != nil {
		if errors.Is(err, nats.ErrBucketNotFound) {
			kv, err = js.CreateKeyValue(&nats.KeyValueConfig{
				Bucket:  SESSION_CACHE_BUCKET,
				History: 1,
				TTL:     5 * time.Minute, // Safety TTL for orphaned entries
			})
			if err != nil {
				return nil, err
			}
		} else {
			// Some other error occurred
			return nil, err
		}
	}

	return &SessionCache{
		kv: kv,
	}, nil
}

func (c *SessionCache) Get(ctx context.Context, visitorID string) (string, error) {
	span, ctx := telemetry.StartServiceSpan(ctx, "SessionCache.Get")
	defer span.Finish()
	span.LogKV("visitorID", visitorID)

	entry, err := c.kv.Get(visitorID)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			span.LogKV("result.found", false)
			return "", nil // Return empty string if not found
		}
		span.TraceError(err)
		return "", err
	}

	// Assuming the value is a string
	var sessionID string
	if err = json.Unmarshal(entry.Value(), &sessionID); err != nil {
		span.TraceError(err)
		return "", err
	}

	span.LogKV("result.sessionID", sessionID)
	return sessionID, nil
}

func (c *SessionCache) Set(ctx context.Context, visitorID, sessionID string) error {
	span, ctx := telemetry.StartServiceSpan(ctx, "SessionCache.Set")
	defer span.Finish()
	span.LogKV("visitorID", visitorID, "sessionID", sessionID)

	if visitorID == "" || sessionID == "" {
		return errors.New("visitorID and sessionID cannot be empty")
	}

	value, err := json.Marshal(sessionID)
	if err != nil {
		span.TraceError(err)
		return err
	}

	_, err = c.kv.Put(visitorID, value)
	return err
}

func (c *SessionCache) Delete(visitorID string) error {
	return c.kv.Delete(visitorID)
}
