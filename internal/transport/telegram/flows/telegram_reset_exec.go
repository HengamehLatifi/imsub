package flows

import (
	"context"
	"imsub/internal/usecase"
)

// handleResetViewerCommand executes viewer data deletion: revokes group access
// and removes all viewer-related Redis keys. Runtime scales with the number
// of linked groups (one kick/unban round-trip per group).
func (c *Controller) handleResetViewerCommand(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	return c.executeResetScope(ctx, telegramUserID, editMsgID, lang, usecase.ResetScopeViewer)
}

// handleResetCreatorCommand deletes creator data and reports a summarized
// result. Does not kick members from the linked Telegram group.
func (c *Controller) handleResetCreatorCommand(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	return c.executeResetScope(ctx, telegramUserID, editMsgID, lang, usecase.ResetScopeCreator)
}

// handleResetBothCommand executes a full reset across both viewer and creator
// scopes. Viewer cleanup (including group kicks) runs first, followed by
// creator data deletion. This is the heaviest reset path.
func (c *Controller) handleResetBothCommand(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	return c.executeResetScope(ctx, telegramUserID, editMsgID, lang, usecase.ResetScopeBoth)
}

func (c *Controller) executeResetScope(ctx context.Context, telegramUserID int64, editMsgID int, lang string, scope usecase.ResetScope) string {
	res, err := c.app.Reset.Execute(ctx, telegramUserID, scope)
	if err != nil {
		view := buildResetErrorView(lang)
		c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
		return view.text
	}
	if res.Empty {
		view := buildResetEmptyView(lang)
		c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
		return ""
	}
	view := buildResetExecutionView(lang, res)
	c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
	return ""
}
