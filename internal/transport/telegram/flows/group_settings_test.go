package flows

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"imsub/internal/platform/i18n"
	"imsub/internal/platform/ratelimit"

	"github.com/mymmrac/telego"
)

func TestCheckGroupSettingsIncludesBotCapabilityWarnings(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	caller := &routeTestCaller{}
	caller.setBotUserID(999)
	caller.setChatMember(999, routeTestAdminMemberJSON(999, true, false, false))
	caller.setChatResult(json.RawMessage(`{"id":-100,"type":"supergroup","title":"VIP","join_by_request":false,"username":"vip_group"}`))
	caller.setChatMemberCount(5)
	caller.setChatAdminsResult(json.RawMessage(`[]`))
	bot, err := telego.NewBot("123456:"+strings.Repeat("a", 35), telego.WithAPICaller(caller))
	if err != nil {
		t.Fatalf("telego.NewBot() error = %v", err)
	}
	limiter := ratelimit.NewRateLimiter(1000, 0)
	t.Cleanup(limiter.Close)

	c := &Controller{
		tg:        bot,
		tgLimiter: limiter,
		store:     &routeTestStore{},
	}

	issues := c.checkGroupSettings(t.Context(), -100, "en")
	if len(issues) < 4 {
		t.Fatalf("checkGroupSettings() returned %d issues, want at least 4; got=%v", len(issues), issues)
	}
	assertContainsIssue(t, issues, i18n.Translate("en", msgGroupWarnBotNoInvite))
	assertContainsIssue(t, issues, i18n.Translate("en", msgGroupWarnBotNoRestrict))
	assertContainsIssue(t, issues, i18n.Translate("en", msgGroupWarnPublic))
	assertContainsIssue(t, issues, i18n.Translate("en", msgGroupWarnJoinByReq))
}

func assertContainsIssue(t *testing.T, issues []string, want string) {
	t.Helper()

	if slices.Contains(issues, want) {
		return
	}
	t.Fatalf("issues = %v, want to contain %q", issues, want)
}
