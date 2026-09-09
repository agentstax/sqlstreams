package diagnostic

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
)

// DiagnosticRecovery states whether an unchanged retry of the operation can succeed.
type DiagnosticRecovery string

const (
	RecoveryTransient DiagnosticRecovery = "transient" // attempt unchanged -> retry can succeed
	RecoveryPermanent DiagnosticRecovery = "permanent" // attempt unchanged -> retry cannot succeed
)

// DiagnosticError is the one error shape:
// - code
// - recovery
// - problem
// - fix
// - diagnose queries fixed at declaration
// - values
// - wrapped cause attached per raise via With and Wrap
type DiagnosticError struct {
	code     string
	recovery DiagnosticRecovery
	problem  string
	fix      string            // "" when the code cannot know the remedy; may carry {attribute} placeholders
	queries  []DiagnosticQuery // none when the condition has no state to look at
	values   []slog.Attr
	wrapped  error
}

// NewDiagnosticError copies queries and registers the completed declaration.
// Structural mistakes panic because declarations are built at package init.
func NewDiagnosticError(code string, recovery DiagnosticRecovery, problem string, fix string, queries ...*DiagnosticQuery) *DiagnosticError {
	if recovery != RecoveryTransient && recovery != RecoveryPermanent {
		panic("recovery must be RecoveryTransient or RecoveryPermanent: " + string(recovery))
	}
	if problem == "" {
		panic("problem must not be empty: " + code)
	}

	declared := &DiagnosticError{code: code, recovery: recovery, problem: problem, fix: fix, queries: copyDiagnosticQueries(queries)}
	register(declared)
	return declared
}

// Recovery says whether an unchanged retry can succeed.
func (e *DiagnosticError) Recovery() DiagnosticRecovery {
	return e.recovery
}

// Problem is the declared fact -- what is wrong and why.
func (e *DiagnosticError) Problem() string {
	return e.problem
}

// Fix is the declared remedy, its {attribute} placeholders unfilled.
func (e *DiagnosticError) Fix() string {
	return e.fix
}

// Queries returns detached query values; editing them does not change the declaration.
func (e *DiagnosticError) Queries() []DiagnosticQuery {
	return slices.Clone(e.queries)
}

// FixPlaceholders lists each attribute name the fix substitutes, once, in
// first-appearance order.
func (e *DiagnosticError) FixPlaceholders() []string {
	return placeholderNames(e.fix)
}

// Fill substitutes text's {attribute} placeholders with the values this raise
// attached -- the declared fix, or a surface's own rewording of it.
func (e *DiagnosticError) Fill(text string) string {
	return fillPlaceholders(text, e.values)
}

// With returns a copy carrying the given name/value pairs appended to any
// already attached.
// Identifier strings render quoted, everything else via its slog value.
func (e *DiagnosticError) With(pairs ...any) *DiagnosticError {
	copied := *e
	copied.values = append(slices.Clone(e.values), toAttributes(pairs)...)
	return &copied
}

// Wrap returns a copy carrying cause as the wrapped error, reachable through
// errors.Is/As and rendered after the code in the one-liner.
func (e *DiagnosticError) Wrap(cause error) *DiagnosticError {
	copied := *e
	copied.wrapped = cause
	return &copied
}

// Values returns the attached name/value pairs in attachment order.
func (e *DiagnosticError) Values() []slog.Attr {
	return slices.Clone(e.values)
}

// Unwrap returns the wrapped cause, nil when none was attached.
func (e *DiagnosticError) Unwrap() error {
	return e.wrapped
}

// Error renders the one-liner:
// - problem: name value, name value -- fix [code]: cause.
// No values drops the ":", an empty fix drops the "--",
// no cause drops the trailing chain.
// The fix's placeholders fill from the attached values.
func (e *DiagnosticError) Error() string {
	var builder strings.Builder
	builder.WriteString(e.problem)

	for i, attribute := range e.values {
		if i == 0 {
			builder.WriteString(": ")
		} else {
			builder.WriteString(", ")
		}
		builder.WriteString(attribute.Key)
		builder.WriteString(" ")
		builder.WriteString(formatValue(attribute.Value))
	}

	if e.fix != "" {
		builder.WriteString(" -- ")
		builder.WriteString(e.Fill(e.fix))
	}

	builder.WriteString(" [")
	builder.WriteString(e.code)
	builder.WriteString("]")

	if e.wrapped != nil {
		builder.WriteString(": ")
		builder.WriteString(e.wrapped.Error())
	}

	return builder.String()
}

// Is matches on code, so errors.Is(raised, pkg.ErrX) holds for every copy
// With and Wrap produce.
func (e *DiagnosticError) Is(target error) bool {
	targetError, ok := target.(*DiagnosticError)
	if !ok {
		return false
	}
	return targetError.code == e.code
}

// LogValue renders the same parts as fields for JSON logs.
func (e *DiagnosticError) LogValue() slog.Value {
	attributes := []slog.Attr{
		slog.String("code", e.code),
		slog.String("problem", e.problem),
		slog.String("recovery", string(e.recovery)),
		slog.String("docs", e.Docs()),
	}

	if e.fix != "" {
		attributes = append(attributes, slog.String("fix", e.Fill(e.fix)))
	}
	attributes = append(attributes, e.values...)
	if e.wrapped != nil {
		attributes = append(attributes, slog.String("cause", e.wrapped.Error()))
	}

	return slog.GroupValue(attributes...)
}

// Docs returns the error's documentation page, derived from the code.
func (e *DiagnosticError) Docs() string {
	return docsBaseURL + e.code
}

// GetCode is the declaration's SS code.
func (e *DiagnosticError) GetCode() string {
	return e.code
}

// GetKind is DiagnosticKindError.
func (e *DiagnosticError) GetKind() DiagnosticKind {
	return DiagnosticKindError
}

// Errors lists every registered error ordered by code.
func Errors() []*DiagnosticError {
	return listRegistered[*DiagnosticError]()
}

// ***************
// *** HELPERS ***
// ***************

func toAttributes(pairs []any) []slog.Attr {
	attributes := make([]slog.Attr, 0, (len(pairs)+1)/2)
	for i := 0; i < len(pairs); i += 2 {
		name := fmt.Sprint(pairs[i])

		// a name with no value is a raise-site bug; render the gap
		// rather than crash or silently drop the name
		if i+1 >= len(pairs) {
			attributes = append(attributes, slog.String(name, "(missing)"))
			break
		}
		attributes = append(attributes, slog.Any(name, pairs[i+1]))
	}
	return attributes
}

func formatValue(value slog.Value) string {
	if value.Kind() == slog.KindString {
		return strconv.Quote(value.String())
	}
	return value.String()
}
