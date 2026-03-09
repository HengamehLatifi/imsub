package usecase

import (
	"context"
	"fmt"

	"imsub/internal/core"
	"imsub/internal/events"
)

const (
	subscriptionEndResultApplied = "applied"
	subscriptionEndResultMissing = "missing"
	subscriptionEndResultFailed  = "failed"
)

type subscriptionEndService interface {
	PrepareEnd(ctx context.Context, broadcasterID, broadcasterLogin, twitchUserID, twitchLogin string) (core.PreparedEnd, error)
}

// SubscriptionEndResult is the application-layer result for a subscription-end event.
type SubscriptionEndResult struct {
	ResultLabel string
	Prepared    core.PreparedEnd
}

// SubscriptionEndUseCase coordinates sub-end preparation and outcome events.
type SubscriptionEndUseCase struct {
	svc    subscriptionEndService
	events events.EventSink
}

// NewSubscriptionEndUseCase builds a subscription-end use case.
func NewSubscriptionEndUseCase(svc subscriptionEndService, sink events.EventSink) *SubscriptionEndUseCase {
	return &SubscriptionEndUseCase{svc: svc, events: events.EnsureSink(sink)}
}

// Prepare resolves transport-ready side effects for a sub-end event.
func (u *SubscriptionEndUseCase) Prepare(ctx context.Context, broadcasterID, broadcasterLogin, twitchUserID, twitchLogin string) (SubscriptionEndResult, error) {
	res, err := u.svc.PrepareEnd(ctx, broadcasterID, broadcasterLogin, twitchUserID, twitchLogin)
	if err != nil {
		u.recordResult(ctx, subscriptionEndResultFailed)
		return SubscriptionEndResult{ResultLabel: subscriptionEndResultFailed}, fmt.Errorf("prepare subscription end: %w", err)
	}
	if !res.Found {
		u.recordResult(ctx, subscriptionEndResultMissing)
		return SubscriptionEndResult{ResultLabel: subscriptionEndResultMissing}, nil
	}
	u.recordResult(ctx, subscriptionEndResultApplied)
	return SubscriptionEndResult{
		ResultLabel: subscriptionEndResultApplied,
		Prepared:    res,
	}, nil
}

func (u *SubscriptionEndUseCase) recordResult(ctx context.Context, result string) {
	u.events.Emit(ctx, events.Event{
		Name:    events.NameSubscriptionEnd,
		Outcome: result,
	})
}
