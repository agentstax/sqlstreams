package consume

import "time"

// BindingOutcome is where one DeclareBindings attempt ended up.
type BindingOutcome string

const (
	BindingInstalled BindingOutcome = "installed" // the declared set is now the group's effective set
	BindingJoined    BindingOutcome = "joined"    // the declared set was already stored
	BindingWaiting   BindingOutcome = "waiting"   // a live instance still declares a different stored set
)

// Binding is one declarer's newest declaration on a group.
// BindingInstalled is the group's effective set.
// BindingJoined is a declarer that found its set already stored.
// BindingWaiting is a declarer still blocked on changing the effective set.
type Binding struct {
	ConsumerGroupName string         `json:"consumer_group"`
	TopicName         string         `json:"topic"`
	Status            BindingOutcome `json:"status"`       // installed, joined, or waiting
	Patterns          []string       `json:"patterns"`     // empty = the whole topic
	DeclaredBy        string         `json:"declared_by"`  // the declaring process (common.ProcessIdentity)
	DeclaredAt        time.Time      `json:"declared_at"`  // when the declarer's Register ran
	AttemptedAt       time.Time      `json:"attempted_at"` // the declarer's latest attempt -- a waiting declarer retries
}
