package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"imsub/internal/core"
)

func parseGroupTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return ts
}

func (s *Store) parseManagedGroup(vals map[string]string, chatID int64) core.ManagedGroup {
	group := core.ManagedGroup{
		ChatID:       chatID,
		CreatorID:    vals["creator_id"],
		GroupName:    vals["group_name"],
		Policy:       core.GroupPolicy(vals["policy"]),
		RegisteredAt: parseGroupTime(vals["registered_at"]),
		UpdatedAt:    parseGroupTime(vals["updated_at"]),
	}
	if group.Policy == "" {
		group.Policy = core.GroupPolicyObserve
	}
	return group
}

// ManagedGroupByChatID returns the managed group for chatID, if present.
func (s *Store) ManagedGroupByChatID(ctx context.Context, chatID int64) (core.ManagedGroup, bool, error) {
	vals, err := s.rdb.HGetAll(ctx, keyManagedGroup(chatID)).Result()
	if err != nil {
		return core.ManagedGroup{}, false, fmt.Errorf("redis hgetall managed group: %w", err)
	}
	if len(vals) == 0 {
		return core.ManagedGroup{}, false, nil
	}
	return s.parseManagedGroup(vals, chatID), true, nil
}

// ListManagedGroupsByCreator returns all managed groups linked to creatorID.
func (s *Store) ListManagedGroupsByCreator(ctx context.Context, creatorID string) ([]core.ManagedGroup, error) {
	ids, err := s.rdb.SMembers(ctx, keyManagedGroupsByCreator(creatorID)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis smembers managed groups by creator: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	out := make([]core.ManagedGroup, 0, len(ids))
	for _, raw := range ids {
		chatID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			s.log().Warn("ListManagedGroupsByCreator invalid chat id, skipping", "creator_id", creatorID, "chat_id_raw", raw, "error", parseErr)
			continue
		}
		group, ok, getErr := s.ManagedGroupByChatID(ctx, chatID)
		if getErr != nil {
			return nil, getErr
		}
		if !ok {
			continue
		}
		out = append(out, group)
	}
	return out, nil
}

// ListManagedGroups returns all managed groups.
func (s *Store) ListManagedGroups(ctx context.Context) ([]core.ManagedGroup, error) {
	ids, err := s.rdb.SMembers(ctx, keyManagedGroupsSet()).Result()
	if err != nil {
		return nil, fmt.Errorf("redis smembers managed groups: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]core.ManagedGroup, 0, len(ids))
	for _, raw := range ids {
		chatID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			s.log().Warn("ListManagedGroups invalid chat id, skipping", "chat_id_raw", raw, "error", parseErr)
			continue
		}
		group, ok, getErr := s.ManagedGroupByChatID(ctx, chatID)
		if getErr != nil {
			return nil, getErr
		}
		if ok {
			out = append(out, group)
		}
	}
	return out, nil
}

// ListTrackedGroupIDsForUser returns tracked group chat IDs for telegramUserID.
func (s *Store) ListTrackedGroupIDsForUser(ctx context.Context, telegramUserID int64) ([]int64, error) {
	rawIDs, err := s.rdb.SMembers(ctx, keyUserTrackedGroups(telegramUserID)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis smembers user tracked groups: %w", err)
	}
	if len(rawIDs) == 0 {
		return nil, nil
	}
	out := make([]int64, 0, len(rawIDs))
	for _, raw := range rawIDs {
		chatID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			s.log().Warn("ListTrackedGroupIDsForUser invalid chat id, skipping", "telegram_user_id", telegramUserID, "chat_id_raw", raw, "error", parseErr)
			continue
		}
		out = append(out, chatID)
	}
	return out, nil
}

// UpsertManagedGroup creates or updates a managed group record and its indices.
func (s *Store) UpsertManagedGroup(ctx context.Context, group core.ManagedGroup) error {
	if group.Policy == "" {
		group.Policy = core.GroupPolicyObserve
	}
	now := time.Now().UTC()
	if group.RegisteredAt.IsZero() {
		group.RegisteredAt = now
	}
	group.UpdatedAt = now

	existing, ok, err := s.ManagedGroupByChatID(ctx, group.ChatID)
	if err != nil {
		return err
	}

	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, keyManagedGroup(group.ChatID), map[string]any{
		"chat_id":       strconv.FormatInt(group.ChatID, 10),
		"creator_id":    group.CreatorID,
		"group_name":    group.GroupName,
		"policy":        string(group.Policy),
		"registered_at": group.RegisteredAt.UTC().Format(time.RFC3339),
		"updated_at":    group.UpdatedAt.UTC().Format(time.RFC3339),
	})
	pipe.SAdd(ctx, keyManagedGroupsSet(), strconv.FormatInt(group.ChatID, 10))
	pipe.SAdd(ctx, keyManagedGroupsByCreator(group.CreatorID), strconv.FormatInt(group.ChatID, 10))
	if ok && existing.CreatorID != "" && existing.CreatorID != group.CreatorID {
		pipe.SRem(ctx, keyManagedGroupsByCreator(existing.CreatorID), strconv.FormatInt(group.ChatID, 10))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis exec upsert managed group: %w", err)
	}
	return nil
}

// DeleteManagedGroup removes a managed group and its tracked/untracked indices.
func (s *Store) DeleteManagedGroup(ctx context.Context, chatID int64) error {
	group, ok, err := s.ManagedGroupByChatID(ctx, chatID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	tracked, err := s.rdb.SMembers(ctx, keyTrackedGroupMembers(chatID)).Result()
	if err != nil {
		return fmt.Errorf("redis smembers tracked group members: %w", err)
	}

	pipe := s.rdb.TxPipeline()
	for _, raw := range tracked {
		tgID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			s.log().Warn("DeleteManagedGroup invalid tracked user id, skipping reverse-index cleanup", "chat_id", chatID, "telegram_user_id_raw", raw, "error", parseErr)
			continue
		}
		pipe.SRem(ctx, keyUserTrackedGroups(tgID), strconv.FormatInt(chatID, 10))
	}
	pipe.Del(ctx, keyManagedGroup(chatID))
	pipe.SRem(ctx, keyManagedGroupsSet(), strconv.FormatInt(chatID, 10))
	pipe.SRem(ctx, keyManagedGroupsByCreator(group.CreatorID), strconv.FormatInt(chatID, 10))
	pipe.Del(ctx, keyTrackedGroupMembers(chatID))
	pipe.Del(ctx, keyUntrackedGroupMembers(chatID))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis exec delete managed group: %w", err)
	}
	return nil
}

// AddTrackedGroupMember records telegramUserID as a tracked member of chatID.
func (s *Store) AddTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64, source string, at time.Time) error {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	tgStr := strconv.FormatInt(telegramUserID, 10)
	chatStr := strconv.FormatInt(chatID, 10)
	metaKey := keyTrackedGroupMemberMeta(chatID, telegramUserID)

	pipe := s.rdb.TxPipeline()
	pipe.SAdd(ctx, keyTrackedGroupMembers(chatID), tgStr)
	pipe.SRem(ctx, keyUntrackedGroupMembers(chatID), tgStr)
	pipe.SAdd(ctx, keyUserTrackedGroups(telegramUserID), chatStr)
	pipe.HSet(ctx, metaKey, map[string]any{
		"state":         "tracked",
		"source":        source,
		"first_seen_at": at.UTC().Format(time.RFC3339),
		"last_seen_at":  at.UTC().Format(time.RFC3339),
		"last_status":   "member",
		"telegram_user": tgStr,
		"group_chat_id": chatStr,
	})
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis exec add tracked group member: %w", err)
	}
	return nil
}

// RemoveTrackedGroupMember removes telegramUserID from the tracked set for chatID.
func (s *Store) RemoveTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) error {
	tgStr := strconv.FormatInt(telegramUserID, 10)
	pipe := s.rdb.TxPipeline()
	pipe.SRem(ctx, keyTrackedGroupMembers(chatID), tgStr)
	pipe.SRem(ctx, keyUserTrackedGroups(telegramUserID), strconv.FormatInt(chatID, 10))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis exec remove tracked group member: %w", err)
	}
	return nil
}

// IsTrackedGroupMember reports whether telegramUserID is tracked in chatID.
func (s *Store) IsTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) (bool, error) {
	res, err := s.rdb.SIsMember(ctx, keyTrackedGroupMembers(chatID), strconv.FormatInt(telegramUserID, 10)).Result()
	if err != nil {
		return false, fmt.Errorf("redis sismember tracked group member: %w", err)
	}
	return res, nil
}

// UpsertUntrackedGroupMember records telegramUserID as observed but untracked in chatID.
func (s *Store) UpsertUntrackedGroupMember(ctx context.Context, chatID, telegramUserID int64, source, status string, at time.Time) error {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	tgStr := strconv.FormatInt(telegramUserID, 10)
	metaKey := keyTrackedGroupMemberMeta(chatID, telegramUserID)
	firstSeen := at.UTC().Format(time.RFC3339)
	existingFirstSeen, _ := s.rdb.HGet(ctx, metaKey, "first_seen_at").Result()
	if existingFirstSeen != "" {
		firstSeen = existingFirstSeen
	}

	pipe := s.rdb.TxPipeline()
	pipe.SAdd(ctx, keyUntrackedGroupMembers(chatID), tgStr)
	pipe.HSet(ctx, metaKey, map[string]any{
		"state":         "untracked",
		"source":        source,
		"first_seen_at": firstSeen,
		"last_seen_at":  at.UTC().Format(time.RFC3339),
		"last_status":   status,
		"telegram_user": tgStr,
		"group_chat_id": strconv.FormatInt(chatID, 10),
	})
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis exec upsert untracked group member: %w", err)
	}
	return nil
}

// RemoveUntrackedGroupMember removes telegramUserID from the untracked set for chatID.
func (s *Store) RemoveUntrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) error {
	if err := s.rdb.SRem(ctx, keyUntrackedGroupMembers(chatID), strconv.FormatInt(telegramUserID, 10)).Err(); err != nil {
		return fmt.Errorf("redis srem untracked group member: %w", err)
	}
	return nil
}

// CountUntrackedGroupMembers returns the number of untracked users seen in chatID.
func (s *Store) CountUntrackedGroupMembers(ctx context.Context, chatID int64) (int, error) {
	count, err := s.rdb.SCard(ctx, keyUntrackedGroupMembers(chatID)).Result()
	if err != nil {
		return 0, fmt.Errorf("redis scard untracked group members: %w", err)
	}
	return int(count), nil
}
