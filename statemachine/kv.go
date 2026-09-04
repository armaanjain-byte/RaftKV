package statemachine

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync"
)

// ErrKeyNotFound is returned when a Get operation looks up a key that does not exist.
var ErrKeyNotFound = errors.New("key not found")

// KVStateMachine implements the StateMachine interface using an in-memory map.
type KVStateMachine struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewKVStateMachine creates a new initialized KVStateMachine.
func NewKVStateMachine() *KVStateMachine {
	return &KVStateMachine{
		data: make(map[string]string),
	}
}

// Apply executes the given command deterministically.
func (kv *KVStateMachine) Apply(cmd Command) Result {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	switch cmd.Op {
	case OpPut:
		kv.data[cmd.Key] = cmd.Value
		return Result{Value: "", Err: nil}
	case OpGet:
		val, ok := kv.data[cmd.Key]
		if !ok {
			return Result{Value: "", Err: ErrKeyNotFound}
		}
		return Result{Value: val, Err: nil}
	default:
		return Result{Value: "", Err: errors.New("unknown command operation")}
	}
}

// Snapshot serializes the KV state into a byte slice using gob encoding.
func (kv *KVStateMachine) Snapshot() ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(kv.data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Restore completely replaces the KV state with the data from the snapshot.
func (kv *KVStateMachine) Restore(data []byte) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var newData map[string]string
	if err := dec.Decode(&newData); err != nil {
		return err
	}
	kv.data = newData
	return nil
}
