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
	ListActiveCreatorGroups(ctx context.Context) ([]ActiveCreatorGroups, error)
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
	active, err := r.store.ListActiveCreatorGroups(ctx)
	if err != nil {
		r.log.Warn("build join targets list active creator groups failed", "error", err)
		return resolvedJoinPlan{}, fmt.Errorf("list active creator groups: %w", err)
	}

	out := resolvedJoinPlan{
		activeCreatorNames: make([]string, 0, len(active)),
		inviteGroups:       make([]resolvedJoinGroup, 0, len(active)),
		untrackedGroups:    make([]int64, 0, len(active)),
	}
	for _, item := range active {
		isSubscriber, err := r.store.IsCreatorSubscriber(ctx, item.Creator.ID, twitchUserID)
		if err != nil {
			r.log.Warn("build join targets is creator subscriber failed", "creator_id", item.Creator.ID, "error", err)
			continue
		}
		if !isSubscriber {
			for _, group := range item.Groups {
				out.untrackedGroups = append(out.untrackedGroups, group.ChatID)
			}
			continue
		}

		out.activeCreatorNames = append(out.activeCreatorNames, item.Creator.Name)
		for _, group := range item.Groups {
			if r.membership.IsGroupMember(ctx, group.ChatID, telegramUserID) {
				continue
			}
			out.inviteGroups = append(out.inviteGroups, resolvedJoinGroup{
				creatorName: item.Creator.Name,
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
