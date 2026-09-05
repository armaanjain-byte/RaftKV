package raft

import "sync"

// Persister holds the persistent state for a Raft node.
type Persister struct {
	mu        sync.Mutex
	raftState []byte
}

func MakePersister() *Persister {
	return &Persister{}
}

// SaveRaftState atomically saves the Raft state (currentTerm, votedFor, etc.).
func (ps *Persister) SaveRaftState(state []byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.raftState = state
}

// ReadRaftState reads the most recently saved Raft state.
func (ps *Persister) ReadRaftState() []byte {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.raftState
}
