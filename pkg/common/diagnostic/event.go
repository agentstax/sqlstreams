package diagnostic

import "slices"

// DiagnosticEvent is a declared operator-actionable log event: the static message
// a call site logs and the code that rides in its "code" attribute.
type DiagnosticEvent struct {
	code    string
	message string
	queries []DiagnosticQuery // none when the event has no state to look at
}

// NewDiagnosticEvent copies queries and registers the completed declaration.
// A non-empty consequence is appended to the message after " -- ".
func NewDiagnosticEvent(code string, message string, consequence string, queries ...*DiagnosticQuery) *DiagnosticEvent {
	if message == "" {
		panic("message must not be empty: " + code)
	}

	if consequence != "" {
		message = message + " -- " + consequence
	}

	declared := &DiagnosticEvent{code: code, message: message, queries: copyDiagnosticQueries(queries)}
	register(declared)
	return declared
}

// Message is the static line a call site logs: the declared message, then
// " -- " and the consequence when one was declared.
func (e *DiagnosticEvent) Message() string {
	return e.message
}

// Queries returns detached query values; editing them does not change the declaration.
func (e *DiagnosticEvent) Queries() []DiagnosticQuery {
	return slices.Clone(e.queries)
}

// Docs returns the event's documentation page, derived from the code.
func (e *DiagnosticEvent) Docs() string {
	return docsBaseURL + e.code
}

// GetCode is the declaration's VK code.
func (e *DiagnosticEvent) GetCode() string {
	return e.code
}

// GetKind is DiagnosticKindEvent.
func (e *DiagnosticEvent) GetKind() DiagnosticKind {
	return DiagnosticKindEvent
}

// Events lists every registered log event ordered by code.
func Events() []*DiagnosticEvent {
	return listRegistered[*DiagnosticEvent]()
}
