package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"

	"imsub/internal/core"
	"imsub/internal/events"
)

type viewerOAuthServiceStub struct {
	linkViewerFn func(context.Context, string, core.OAuthStatePayload, string) (core.ViewerResult, error)
}

func (s viewerOAuthServiceStub) LinkViewer(ctx context.Context, code string, payload core.OAuthStatePayload, lang string) (core.ViewerResult, error) {
	return s.linkViewerFn(ctx, code, payload, lang)
}

type viewerOAuthObserverStub struct {
	events []events.Event
}

func (o *viewerOAuthObserverStub) Emit(_ context.Context, evt events.Event) {
	o.events = append(o.events, evt)
}

func TestViewerOAuthCompleteSuccess(t *testing.T) {
	t.Parallel()

	obs := &viewerOAuthObserverStub{}
	uc := NewViewerOAuthUseCase(viewerOAuthServiceStub{
		linkViewerFn: func(context.Context, string, core.OAuthStatePayload, string) (core.ViewerResult, error) {
			return core.ViewerResult{
				TwitchUserID:      "tw-1",
				TwitchLogin:       "viewer",
				TwitchDisplayName: "Viewer",
				DisplacedUserID:   42,
			}, nil
		},
	}, obs)

	got, err := uc.Complete(t.Context(), "code", core.OAuthStatePayload{TelegramUserID: 7}, "en")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got.ResultLabel != viewerOAuthResultSuccess || got.TwitchLogin != "viewer" || got.DisplacedUserID != 42 {
		t.Fatalf("got = %+v", got)
	}
	want := []events.Event{{Name: events.NameViewerOAuth, Outcome: viewerOAuthResultSuccess}}
	if !slices.EqualFunc(obs.events, want, equalEvents) {
		t.Fatalf("events = %+v, want %+v", obs.events, want)
	}
}

func TestViewerOAuthCompleteMapsFlowErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "token_exchange", err: &core.FlowError{Kind: core.KindTokenExchange, Cause: errors.New("boom")}, want: viewerOAuthResultTokenExchangeFailed},
		{name: "user_info", err: &core.FlowError{Kind: core.KindUserInfo, Cause: errors.New("boom")}, want: viewerOAuthResultUserInfoFailed},
		{name: "save", err: &core.FlowError{Kind: core.KindSave, Cause: errors.New("boom")}, want: viewerOAuthResultSaveFailed},
		{name: "store", err: &core.FlowError{Kind: core.KindStore, Cause: errors.New("boom")}, want: viewerOAuthResultSaveFailed},
		{name: "unknown", err: errors.New("boom"), want: viewerOAuthResultSaveFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obs := &viewerOAuthObserverStub{}
			uc := NewViewerOAuthUseCase(viewerOAuthServiceStub{
				linkViewerFn: func(context.Context, string, core.OAuthStatePayload, string) (core.ViewerResult, error) {
					return core.ViewerResult{}, tc.err
				},
			}, obs)

			got, err := uc.Complete(t.Context(), "code", core.OAuthStatePayload{TelegramUserID: 7}, "en")
			if err == nil {
				t.Fatal("Complete() error = nil, want non-nil")
			}
			if got.ResultLabel != tc.want {
				t.Fatalf("ResultLabel = %q, want %q", got.ResultLabel, tc.want)
			}
			want := []events.Event{{Name: events.NameViewerOAuth, Outcome: tc.want}}
			if !slices.EqualFunc(obs.events, want, equalEvents) {
				t.Fatalf("events = %+v, want %+v", obs.events, want)
			}
		})
	}
}
