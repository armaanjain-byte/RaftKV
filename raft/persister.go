package raft

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync"
)

// ErrCrashInjected is returned by a Persister whose crash hook fires.
var ErrCrashInjected = errors.New("crash injected by test")

// persistentState is the canonical serialized representation of the
// (currentTerm, votedFor) pair. Using a single struct guarantees that
// gob always writes BOTH fields in ONE Encode call, so a partial write
// can only produce either the full old blob or the full new blob — never
// a mixed pair (e.g., new term with old votedFor).
type persistentState struct {
	CurrentTerm int
	VotedFor    int
}

// Persister provides durable storage for a Raft node's persistent state.
// SaveRaftState is the only write path; it is intentionally a single atomic call.
type Persister struct {
	mu         sync.Mutex
	raftState  []byte
	// beforeSave, if non-nil, is called BEFORE the state is actually written.
	// If it panics, the write does not happen, simulating a crash between
	// in-memory update and the durable write.
	beforeSave func()
}

func MakePersister() *Persister {
	return &Persister{}
}

// SetBeforeSaveHook installs a test hook that fires before every SaveRaftState.
// Tests use this to simulate a crash between in-memory update and durable persistence.
// Set to nil to disable.
func (ps *Persister) SetBeforeSaveHook(fn func()) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.beforeSave = fn
}

// SaveRaftState atomically saves the encoded (currentTerm, votedFor) pair.
// The caller must pass a fully-encoded blob (see EncodeRaftState).
// This is the ONLY write path; there is intentionally no partial-field write.
func (ps *Persister) SaveRaftState(state []byte) {
	ps.mu.Lock()
	hook := ps.beforeSave
	ps.mu.Unlock()

	// Fire the hook OUTSIDE the lock so it can safely call ReadRaftState.
	// If the hook panics, the durable state is NOT updated (old value preserved).
	if hook != nil {
		hook()
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.raftState = state
}

// ReadRaftState reads the most recently saved Raft state.
func (ps *Persister) ReadRaftState() []byte {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.raftState == nil {
		return nil
	}
	cp := make([]byte, len(ps.raftState))
	copy(cp, ps.raftState)
	return cp
}

// EncodeRaftState serializes the (term, votedFor) pair as a single gob blob.
func EncodeRaftState(term, votedFor int) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	// Single Encode call: both fields written atomically into one blob.
	if err := enc.Encode(persistentState{CurrentTerm: term, VotedFor: votedFor}); err != nil {
		panic("raft: failed to encode persistent state: " + err.Error())
	}
	return buf.Bytes()
}

// DecodeRaftState deserializes a blob produced by EncodeRaftState.
// Returns (0, -1) on empty/nil input (fresh start with no prior term/vote).
func DecodeRaftState(data []byte) (term, votedFor int) {
	term = 0
	votedFor = -1
	if len(data) == 0 {
		return
	}
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var ps persistentState
	if err := dec.Decode(&ps); err != nil {
		// Corrupt data treated as fresh start; caller should log this.
		return
	}
	return ps.CurrentTerm, ps.VotedFor
}
