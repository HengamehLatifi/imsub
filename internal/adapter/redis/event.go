package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// --- Event dedup ---

// EventProcessed checks whether an EventSub message ID has already been recorded.
func (s *Store) EventProcessed(ctx context.Context, messageID string) (bool, error) {
	exists, err := s.rdb.Exists(ctx, keyEventMessage(messageID)).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists event message: %w", err)
	}
	return exists > 0, nil
}

// MarkEventProcessed uses SET NX to deduplicate events, returning true if already processed.
func (s *Store) MarkEventProcessed(ctx context.Context, messageID string, ttl time.Duration) (alreadyProcessed bool, err error) {
	err = s.rdb.SetArgs(ctx, keyEventMessage(messageID), "1", redis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Err()
	if errors.Is(err, redis.Nil) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis set nx event message: %w", err)
	}
	return false, nil
}
