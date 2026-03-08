package flows

import "strings"

type callbackDomain string

const (
	callbackDomainViewer  callbackDomain = "viewer"
	callbackDomainCreator callbackDomain = "creator"
	callbackDomainReset   callbackDomain = "reset"
)

type callbackVerb string

const (
	callbackVerbRefresh   callbackVerb = "refresh"
	callbackVerbRegister  callbackVerb = "register"
	callbackVerbReconnect callbackVerb = "reconnect"
	callbackVerbOpen      callbackVerb = "open"
	callbackVerbPick      callbackVerb = "pick"
	callbackVerbBack      callbackVerb = "back"
	callbackVerbMenu      callbackVerb = "menu"
	callbackVerbCancel    callbackVerb = "cancel"
	callbackVerbExecute   callbackVerb = "exec"
)

type resetOrigin string

const (
	resetOriginViewer  resetOrigin = "viewer"
	resetOriginCreator resetOrigin = "creator"
	resetOriginCommand resetOrigin = "command"
)

type resetScope string

const (
	resetScopeViewer  resetScope = "viewer"
	resetScopeCreator resetScope = "creator"
	resetScopeBoth    resetScope = "both"
)

type callbackAction struct {
	domain callbackDomain
	verb   callbackVerb
	origin resetOrigin
	scope  resetScope
}

func (a callbackAction) String() string {
	parts := []string{string(a.domain), string(a.verb)}
	if a.origin != "" {
		parts = append(parts, string(a.origin))
	}
	if a.scope != "" {
		parts = append(parts, string(a.scope))
	}
	return strings.Join(parts, ":")
}

func parseCallbackAction(data string) (callbackAction, bool) {
	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		return callbackAction{}, false
	}

	action := callbackAction{
		domain: callbackDomain(parts[0]),
		verb:   callbackVerb(parts[1]),
	}
	switch action.domain {
	case callbackDomainViewer:
		if len(parts) != 2 || action.verb != callbackVerbRefresh {
			return callbackAction{}, false
		}
		return action, true
	case callbackDomainCreator:
		if len(parts) != 2 {
			return callbackAction{}, false
		}
		switch action.verb {
		case callbackVerbRefresh, callbackVerbRegister, callbackVerbReconnect:
			return action, true
		case callbackVerbOpen, callbackVerbPick, callbackVerbBack, callbackVerbMenu, callbackVerbCancel, callbackVerbExecute:
			return callbackAction{}, false
		default:
			return callbackAction{}, false
		}
	case callbackDomainReset:
		switch action.verb {
		case callbackVerbOpen, callbackVerbBack, callbackVerbMenu, callbackVerbCancel:
			if len(parts) != 3 {
				return callbackAction{}, false
			}
			action.origin = resetOrigin(parts[2])
			if !action.origin.valid() {
				return callbackAction{}, false
			}
			return action, true
		case callbackVerbPick, callbackVerbExecute:
			if len(parts) != 4 {
				return callbackAction{}, false
			}
			action.origin = resetOrigin(parts[2])
			action.scope = resetScope(parts[3])
			if !action.origin.valid() || !action.scope.valid() {
				return callbackAction{}, false
			}
			return action, true
		case callbackVerbRefresh, callbackVerbRegister, callbackVerbReconnect:
			return callbackAction{}, false
		default:
			return callbackAction{}, false
		}
	default:
		return callbackAction{}, false
	}
}

func (o resetOrigin) valid() bool {
	switch o {
	case resetOriginViewer, resetOriginCreator, resetOriginCommand:
		return true
	default:
		return false
	}
}

func (s resetScope) valid() bool {
	switch s {
	case resetScopeViewer, resetScopeCreator, resetScopeBoth:
		return true
	default:
		return false
	}
}

func viewerRefreshCallback() string {
	return callbackAction{domain: callbackDomainViewer, verb: callbackVerbRefresh}.String()
}

func creatorRefreshCallback() string {
	return callbackAction{domain: callbackDomainCreator, verb: callbackVerbRefresh}.String()
}

func creatorReconnectCallback() string {
	return callbackAction{domain: callbackDomainCreator, verb: callbackVerbReconnect}.String()
}

func resetOpenCallback(origin resetOrigin) string {
	return callbackAction{domain: callbackDomainReset, verb: callbackVerbOpen, origin: origin}.String()
}

func resetPickCallback(origin resetOrigin, scope resetScope) string {
	return callbackAction{domain: callbackDomainReset, verb: callbackVerbPick, origin: origin, scope: scope}.String()
}

func resetBackCallback(origin resetOrigin) string {
	return callbackAction{domain: callbackDomainReset, verb: callbackVerbBack, origin: origin}.String()
}

func resetMenuCallback(origin resetOrigin) string {
	return callbackAction{domain: callbackDomainReset, verb: callbackVerbMenu, origin: origin}.String()
}

func resetCancelCallback(origin resetOrigin) string {
	return callbackAction{domain: callbackDomainReset, verb: callbackVerbCancel, origin: origin}.String()
}

func resetExecuteCallback(origin resetOrigin, scope resetScope) string {
	return callbackAction{domain: callbackDomainReset, verb: callbackVerbExecute, origin: origin, scope: scope}.String()
}
