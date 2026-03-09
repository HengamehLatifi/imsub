package core

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"imsub/internal/events"
)

type resolvedJoinGroup struct {
	creatorName string
	group       ManagedGroup
}

type resolvedJoinPlan struct {
	activeCreatorNames []string
	inviteGroups       []resolvedJoinGroup
	untrackedGroups    []int64
}

type viewerResolverStore interface {
	ListActiveCreators(ctx context.Context) ([]Creator, error)
	ListManagedGroupsByCreator(ctx context.Context, creatorID string) ([]ManagedGroup, error)
	IsCreatorSubscriber(ctx context.Context, creatorID, twitchUserID string) (bool, error)
}

type viewerMembershipChecker interface {
	IsGroupMember(ctx context.Context, groupChatID, telegramUserID int64) bool
}

type viewerEligibilityResolver struct {
	store      viewerResolverStore
	membership viewerMembershipChecker
	log        *slog.Logger
	obs        events.EventSink
}

func newViewerEligibilityResolver(store viewerResolverStore, membership viewerMembershipChecker, logger *slog.Logger, obs events.EventSink) *viewerEligibilityResolver {
	return &viewerEligibilityResolver{
		store:      store,
		membership: membership,
		log:        logger,
		obs:        obs,
	}
}

func (r *viewerEligibilityResolver) resolve(ctx context.Context, telegramUserID int64, twitchUserID string) (resolvedJoinPlan, error) {
	creators, err := r.store.ListActiveCreators(ctx)
	if err != nil {
		r.log.Warn("build join targets list active creators failed", "error", err)
		return resolvedJoinPlan{}, fmt.Errorf("list active creators: %w", err)
	}

	out := resolvedJoinPlan{
		activeCreatorNames: make([]string, 0, len(creators)),
		inviteGroups:       make([]resolvedJoinGroup, 0, len(creators)),
		untrackedGroups:    make([]int64, 0, len(creators)),
	}
	for _, creator := range creators {
		groups, err := r.store.ListManagedGroupsByCreator(ctx, creator.ID)
		if err != nil {
			r.log.Warn("build join targets list managed groups failed", "creator_id", creator.ID, "error", err)
			continue
		}
		isSubscriber, err := r.store.IsCreatorSubscriber(ctx, creator.ID, twitchUserID)
		if err != nil {
			r.log.Warn("build join targets is creator subscriber failed", "creator_id", creator.ID, "error", err)
			continue
		}
		if !isSubscriber {
			for _, group := range groups {
				out.untrackedGroups = append(out.untrackedGroups, group.ChatID)
			}
			continue
		}

		out.activeCreatorNames = append(out.activeCreatorNames, creator.Name)
		for _, group := range groups {
			if r.membership.IsGroupMember(ctx, group.ChatID, telegramUserID) {
				continue
			}
			out.inviteGroups = append(out.inviteGroups, resolvedJoinGroup{
				creatorName: creator.Name,
				group:       group,
			})
		}
	}

	slices.Sort(out.activeCreatorNames)
	r.recordJoinTargets(ctx, "active_creators", len(out.activeCreatorNames))
	r.recordJoinTargets(ctx, "invite_groups", len(out.inviteGroups))
	return out, nil
}

func (r *viewerEligibilityResolver) recordJoinTargets(ctx context.Context, kind string, count int) {
	if r == nil || r.obs == nil || count <= 0 {
		return
	}
	r.obs.Emit(ctx, events.Event{
		Name:   events.NameViewerJoinTarget,
		Fields: map[string]string{"kind": kind},
		Count:  count,
	})
}
