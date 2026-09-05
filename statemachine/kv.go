package statemachine

import (
	"bytes"
	"encoding/gob"
	"errors"
	"strconv"
	"sync"
)

// ErrKeyNotFound is returned when a Get operation looks up a key that does not exist.
var ErrKeyNotFound = errors.New("key not found")

// Session tracks all applied commands and their results for a given client
// to support idempotent replay of ANY previously-seen sequence number.
type Session struct {
	Results map[int64]Result
}

// kvState represents the entire state of the KV machine that must be snapshot/restored.
type kvState struct {
	Data         map[string]string
	Sessions     map[int64]Session
	NextClientID int64
}

// KVStateMachine implements the StateMachine interface using an in-memory map and dedup table.
type KVStateMachine struct {
	mu    sync.RWMutex
	state kvState
}

// NewKVStateMachine creates a new initialized KVStateMachine.
func NewKVStateMachine() *KVStateMachine {
	return &KVStateMachine{
		state: kvState{
			Data:         make(map[string]string),
			Sessions:     make(map[int64]Session),
			NextClientID: 1, // Start IDs at 1
		},
	}
}

// Apply executes the given command deterministically, ensuring exactly-once semantics per sequence number.
func (kv *KVStateMachine) Apply(cmd Command) Result {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Special case for Register, which establishes a new session
	if cmd.Op == OpRegister {
		newID := kv.state.NextClientID
		kv.state.NextClientID++
		kv.state.Sessions[newID] = Session{Results: make(map[int64]Result)}
		return Result{Value: strconv.FormatInt(newID, 10), Err: nil}
	}

	// For standard operations, verify dedup table for idempotent replay
	session, exists := kv.state.Sessions[cmd.ClientID]
	if exists {
		if res, alreadyApplied := session.Results[cmd.SeqNum]; alreadyApplied {
			return res // Idempotent replay of previously-seen sequence number
		}
	}

	var res Result
	switch cmd.Op {
	case OpPut:
		kv.state.Data[cmd.Key] = cmd.Value
		res = Result{Value: "", Err: nil}
	case OpGet:
		val, ok := kv.state.Data[cmd.Key]
		if !ok {
			res = Result{Value: "", Err: ErrKeyNotFound}
		} else {
			res = Result{Value: val, Err: nil}
		}
	case OpDelete:
		delete(kv.state.Data, cmd.Key)
		res = Result{Value: "", Err: nil}
	default:
		res = Result{Value: "", Err: errors.New("unknown command operation")}
	}

	// Update session tracking if a valid client is provided
	if cmd.ClientID != 0 {
		if !exists {
			session = Session{Results: make(map[int64]Result)}
			kv.state.Sessions[cmd.ClientID] = session
		}
		session.Results[cmd.SeqNum] = res
	}

	return res
}

// Snapshot serializes the KV data and dedup session state into a byte slice.
func (kv *KVStateMachine) Snapshot() ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(kv.state); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Restore completely replaces the KV state and dedup sessions with the data from the snapshot.
func (kv *KVStateMachine) Restore(data []byte) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var newState kvState
	if err := dec.Decode(&newState); err != nil {
		return err
	}
	kv.state = newState
	return nil
}
