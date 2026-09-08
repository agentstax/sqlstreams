package scenario

import (
	"errors"
	"fmt"

	"github.com/agentstax/vulkan/pkg/topic"
)

// TopicDeclaration is one topic under test and the consumer groups on it.
// Every topic runs the scenario's producer phases at the phase's rate.
type TopicDeclaration struct {
	Name            string
	DeliveryLogMode topic.DeliveryLogMode
	Groups          []GroupDeclaration
}

func (t TopicDeclaration) Validate() error {
	if t.Name == "" {
		return errors.New("Name is required")
	}

	// unbucketed reads delivery_log success rows, which only mode all writes
	if t.DeliveryLogMode != topic.DeliveryLogModeAll {
		return fmt.Errorf("DeliveryLogMode must be %q, got %q", topic.DeliveryLogModeAll, t.DeliveryLogMode)
	}
	if len(t.Groups) == 0 {
		return errors.New("Groups must not be empty")
	}
	names := map[string]bool{}
	for i, group := range t.Groups {
		if err := group.Validate(); err != nil {
			return fmt.Errorf("Groups[%d]: %w", i, err)
		}
		if names[group.Name] {
			return fmt.Errorf("Groups[%d].Name already declared: %q", i, group.Name)
		}
		names[group.Name] = true
	}
	return nil
}

// GroupDeclaration is one consumer group: its name, the share of handler
// invocations the lab fails on purpose, the retries before a message is
// dead-lettered, and the messages each instance claims per poll -- 0 leaves
// the library's default, which a quiet run keeps and a ladder raises.
type GroupDeclaration struct {
	Name            string
	HandlerFailRate float64
	MaxRetries      int
	BatchLimit      int
}

func (g GroupDeclaration) Validate() error {
	if g.Name == "" {
		return errors.New("Name is required")
	}
	if g.HandlerFailRate < 0 || g.HandlerFailRate > 1 {
		return fmt.Errorf("HandlerFailRate must be between 0 and 1, got %g", g.HandlerFailRate)
	}
	if g.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries must be >= 0, got %d", g.MaxRetries)
	}
	if g.BatchLimit < 0 {
		return fmt.Errorf("BatchLimit must be >= 0, got %d", g.BatchLimit)
	}
	return nil
}

// String is the [input] consumers line after the group's name.
func (g GroupDeclaration) String() string {
	line := fmt.Sprintf("handler fail rate %g, %d retries then dead", g.HandlerFailRate, g.MaxRetries)
	if g.BatchLimit > 0 {
		line += fmt.Sprintf(", batch %d", g.BatchLimit)
	}
	return line
}
