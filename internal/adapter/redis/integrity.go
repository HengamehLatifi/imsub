package redis

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"imsub/internal/core"

	"github.com/redis/go-redis/v9"
)

// --- Integrity ---

// RepairUserCreatorReverseIndex audits and repairs user↔creator reverse-index sets.
func (s *Store) RepairUserCreatorReverseIndex(ctx context.Context, creators []core.Creator) (indexUsers int, repairedUsers int, missingLinks int, staleLinks int, err error) {
	creatorIDs := make([]string, 0, len(creators))
	for _, c := range creators {
		creatorIDs = append(creatorIDs, c.ID)
	}
	slices.Sort(creatorIDs)

	memberPipe := s.rdb.Pipeline()
	memberCmds := make([]*redis.StringSliceCmd, len(creatorIDs))
	for i, creatorID := range creatorIDs {
		memberCmds[i] = memberPipe.SMembers(ctx, keyCreatorMembers(creatorID))
	}
	if _, err := memberPipe.Exec(ctx); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("redis exec integrity audit member cmds: %w", err)
	}

	wantedByUser := make(map[string]map[string]struct{})
	for i, creatorID := range creatorIDs {
		memberIDs, cmdErr := memberCmds[i].Result()
		if cmdErr != nil {
			return 0, 0, 0, 0, fmt.Errorf("redis result integrity audit member cmds: %w", cmdErr)
		}
		for _, memberID := range memberIDs {
			if _, parseErr := strconv.ParseInt(memberID, 10, 64); parseErr != nil {
				s.log().Warn("integrity audit skipping non-numeric creator member", "creator_id", creatorID, "member_raw", memberID, "error", parseErr)
				continue
			}
			set, ok := wantedByUser[memberID]
			if !ok {
				set = make(map[string]struct{})
				wantedByUser[memberID] = set
			}
			set[creatorID] = struct{}{}
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
		reverseCmds = append(reverseCmds, reversePipe.SMembers(ctx, keyUserCreators(tgID)))
	}
	if len(reverseCmds) > 0 {
		if _, err := reversePipe.Exec(ctx); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("redis exec integrity audit reverse cmds: %w", err)
		}
	}

	writePipe := s.rdb.TxPipeline()
	needsWrite := false
	for i, userIDRaw := range validUserIDRaw {
		currentCreators, cmdErr := reverseCmds[i].Result()
		if cmdErr != nil {
			return 0, 0, 0, 0, fmt.Errorf("redis result integrity audit reverse cmds: %w", cmdErr)
		}
		current := make(map[string]struct{}, len(currentCreators))
		for _, creatorID := range currentCreators {
			current[creatorID] = struct{}{}
		}
		wanted := wantedByUser[userIDRaw]

		userNeedsRepair := false
		for creatorID := range wanted {
			if _, ok := current[creatorID]; !ok {
				missingLinks++
				userNeedsRepair = true
			}
		}
		for creatorID := range current {
			if _, ok := wanted[creatorID]; !ok {
				staleLinks++
				userNeedsRepair = true
			}
		}
		if !userNeedsRepair {
			continue
		}

		repairedUsers++
		needsWrite = true
		key := keyUserCreators(validUserIDs[i])
		writePipe.Del(ctx, key)
		if len(wanted) == 0 {
			continue
		}
		args := make([]any, 0, len(wanted))
		for creatorID := range wanted {
			args = append(args, creatorID)
		}
		writePipe.SAdd(ctx, key, args...)
	}
	if needsWrite {
		if _, err := writePipe.Exec(ctx); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("redis exec integrity audit repair writes: %w", err)
		}
	}

	return len(validUserIDRaw), repairedUsers, missingLinks, staleLinks, nil
}

// ActiveCreatorIDsWithoutGroup counts creators marked active but missing a bound group.
func (s *Store) ActiveCreatorIDsWithoutGroup(ctx context.Context, creators []core.Creator) (int, error) {
	activeIDs, err := s.rdb.SMembers(ctx, keyActiveCreatorsSet()).Result()
	if err != nil {
		return 0, fmt.Errorf("redis smembers active creators: %w", err)
	}
	activeSet := make(map[string]struct{}, len(activeIDs))
	for _, id := range activeIDs {
		activeSet[id] = struct{}{}
	}
	count := 0
	for _, c := range creators {
		if _, ok := activeSet[c.ID]; ok && c.GroupChatID == 0 {
			count++
		}
	}
	return count, nil
}
