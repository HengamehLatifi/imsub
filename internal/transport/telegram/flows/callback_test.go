package flows

import "testing"

func TestParseCallbackAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want callbackAction
		ok   bool
	}{
		{
			name: "viewer refresh",
			data: viewerRefreshCallback(),
			want: callbackAction{domain: callbackDomainViewer, verb: callbackVerbRefresh},
			ok:   true,
		},
		{
			name: "creator reconnect",
			data: creatorReconnectCallback(),
			want: callbackAction{domain: callbackDomainCreator, verb: callbackVerbReconnect},
			ok:   true,
		},
		{
			name: "reset pick",
			data: resetPickCallback(resetOriginCreator, resetScopeBoth),
			want: callbackAction{
				domain: callbackDomainReset,
				verb:   callbackVerbPick,
				origin: resetOriginCreator,
				scope:  resetScopeBoth,
			},
			ok: true,
		},
		{
			name: "reset cancel",
			data: resetCancelCallback(resetOriginCommand),
			want: callbackAction{
				domain: callbackDomainReset,
				verb:   callbackVerbCancel,
				origin: resetOriginCommand,
			},
			ok: true,
		},
		{
			name: "reset menu",
			data: resetMenuCallback(resetOriginViewer),
			want: callbackAction{
				domain: callbackDomainReset,
				verb:   callbackVerbMenu,
				origin: resetOriginViewer,
			},
			ok: true,
		},
		{name: "missing parts", data: "reset", ok: false},
		{name: "invalid origin", data: "reset:open:oops", ok: false},
		{name: "invalid scope", data: "reset:pick:viewer:oops", ok: false},
		{name: "invalid creator verb", data: "creator:oops", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseCallbackAction(tt.data)
			if ok != tt.ok {
				t.Fatalf("parseCallbackAction(%q) ok = %t, want %t", tt.data, ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got != tt.want {
				t.Fatalf("parseCallbackAction(%q) = %+v, want %+v", tt.data, got, tt.want)
			}
		})
	}
}
