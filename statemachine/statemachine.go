package statemachine

// CommandOp represents the type of operation to perform on the state machine.
type CommandOp string

const (
	OpPut CommandOp = "Put"
	OpGet CommandOp = "Get"
)

// Command represents a deterministic operation to be applied to the state machine.
type Command struct {
	Op    CommandOp
	Key   string
	Value string
}

// Result represents the outcome of applying a Command to the state machine.
type Result struct {
	Value string
	Err   error
}

// StateMachine represents the deterministic execution core of a node.
// It must only be mutated via the Apply method to ensure deterministic transitions.
type StateMachine interface {
	// Apply applies a committed command to the state machine.
	Apply(cmd Command) Result

	// Snapshot serializes the current state deterministically.
	Snapshot() ([]byte, error)

	// Restore deserializes state from a snapshot, replacing the current state entirely.
	Restore(data []byte) error
}
