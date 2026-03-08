package redis

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"imsub/internal/core"

	"github.com/redis/go-redis/v9"
)

// RepairUserCreatorReverseIndex audits and repairs tracked-group reverse-index sets.
func (s *Store) RepairUserCreatorReverseIndex(ctx context.Context, creators []core.Creator) (indexUsers int, repairedUsers int, missingLinks int, staleLinks int, err error) {
	_ = creators

	groups, err := s.ListManagedGroups(ctx)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("list managed groups: %w", err)
	}
	if len(groups) == 0 {
		return 0, 0, 0, 0, nil
	}

	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ChatID)
	}
	slices.Sort(groupIDs)

	memberPipe := s.rdb.Pipeline()
	memberCmds := make([]*redis.StringSliceCmd, len(groupIDs))
	for i, groupID := range groupIDs {
		memberCmds[i] = memberPipe.SMembers(ctx, keyTrackedGroupMembers(groupID))
	}
	if _, err := memberPipe.Exec(ctx); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("redis exec integrity audit tracked members: %w", err)
	}

	wantedByUser := make(map[string]map[string]struct{})
	for i, groupID := range groupIDs {
		memberIDs, cmdErr := memberCmds[i].Result()
		if cmdErr != nil {
			return 0, 0, 0, 0, fmt.Errorf("redis result integrity audit tracked members: %w", cmdErr)
		}
		groupIDStr := strconv.FormatInt(groupID, 10)
		for _, memberID := range memberIDs {
			if _, parseErr := strconv.ParseInt(memberID, 10, 64); parseErr != nil {
				s.log().Warn("integrity audit skipping non-numeric tracked member", "group_chat_id", groupID, "member_raw", memberID, "error", parseErr)
				continue
			}
			set, ok := wantedByUser[memberID]
			if !ok {
				set = make(map[string]struct{})
				wantedByUser[memberID] = set
			}
			set[groupIDStr] = struct{}{}
		}
	}

	usersSet, err := s.rdb.SMembers(ctx, keyUsersSet()).Result()
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("redis smembers users set: %w", err)
	}
	userIDs := make([]string, 0, len(usersSet)+len(wantedByUser))
	seenUsers := make(map[string]struct{}, len(usersSet)+len(wantedByUser))
	for _, userID := range usersSet {
		if _, ok := seenUsers[userID]; ok {
			continue
		}
		seenUsers[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	for userID := range wantedByUser {
		if _, ok := seenUsers[userID]; ok {
			continue
		}
		seenUsers[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	slices.Sort(userIDs)

	reversePipe := s.rdb.Pipeline()
	reverseCmds := make([]*redis.StringSliceCmd, 0, len(userIDs))
	validUserIDs := make([]int64, 0, len(userIDs))
	validUserIDRaw := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		tgID, parseErr := strconv.ParseInt(userID, 10, 64)
		if parseErr != nil {
			s.log().Warn("integrity audit skipping non-numeric user id", "user_id_raw", userID, "error", parseErr)
			continue
		}
		validUserIDs = append(validUserIDs, tgID)
		validUserIDRaw = append(validUserIDRaw, userID)
		reverseCmds = append(reverseCmds, reversePipe.SMembers(ctx, keyUserTrackedGroups(tgID)))
	}
	if len(reverseCmds) > 0 {
		if _, err := reversePipe.Exec(ctx); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("redis exec integrity audit tracked reverse index: %w", err)
		}
	}

	writePipe := s.rdb.TxPipeline()
	needsWrite := false
	for i, userIDRaw := range validUserIDRaw {
		currentGroupIDs, cmdErr := reverseCmds[i].Result()
		if cmdErr != nil {
			return 0, 0, 0, 0, fmt.Errorf("redis result integrity audit tracked reverse index: %w", cmdErr)
		}
		current := make(map[string]struct{}, len(currentGroupIDs))
		for _, groupID := range currentGroupIDs {
			current[groupID] = struct{}{}
		}
		wanted := wantedByUser[userIDRaw]

		userNeedsRepair := false
		for groupID := range wanted {
			if _, ok := current[groupID]; !ok {
				missingLinks++
				userNeedsRepair = true
			}
		}
		for groupID := range current {
			if _, ok := wanted[groupID]; !ok {
				staleLinks++
				userNeedsRepair = true
			}
		}
		if !userNeedsRepair {
			continue
		}

		repairedUsers++
		needsWrite = true
		key := keyUserTrackedGroups(validUserIDs[i])
		writePipe.Del(ctx, key)
		if len(wanted) == 0 {
			continue
		}
		args := make([]any, 0, len(wanted))
		for groupID := range wanted {
			args = append(args, groupID)
		}
		writePipe.SAdd(ctx, key, args...)
	}
	if needsWrite {
		if _, err := writePipe.Exec(ctx); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("redis exec integrity audit tracked reverse-index repair: %w", err)
		}
	}

	return len(validUserIDRaw), repairedUsers, missingLinks, staleLinks, nil
}

// ActiveCreatorIDsWithoutGroup counts creators that have no managed groups.
func (s *Store) ActiveCreatorIDsWithoutGroup(ctx context.Context, creators []core.Creator) (int, error) {
	groups, err := s.ListManagedGroups(ctx)
	if err != nil {
		return 0, fmt.Errorf("list managed groups: %w", err)
	}
	managedByCreator := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		managedByCreator[group.CreatorID] = struct{}{}
	}

	count := 0
	for _, creator := range creators {
		if _, ok := managedByCreator[creator.ID]; !ok {
			count++
		}
	}
	return count, nil
}
