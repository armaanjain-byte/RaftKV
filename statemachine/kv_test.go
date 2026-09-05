package statemachine

import (
	"bytes"
	"strconv"
	"testing"
)

func TestKVStateMachine_Apply(t *testing.T) {
	sm := NewKVStateMachine()

	// Test Get on empty store
	res := sm.Apply(Command{Op: OpGet, Key: "k1"})
	if res.Err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", res.Err)
	}

	// Test Put
	res = sm.Apply(Command{Op: OpPut, Key: "k1", Value: "v1"})
	if res.Err != nil {
		t.Fatalf("unexpected error on Put: %v", res.Err)
	}

	// Test Get after Put
	res = sm.Apply(Command{Op: OpGet, Key: "k1"})
	if res.Err != nil {
		t.Fatalf("unexpected error on Get: %v", res.Err)
	}
	if res.Value != "v1" {
		t.Fatalf("expected value 'v1', got '%s'", res.Value)
	}

	// Test Update
	res = sm.Apply(Command{Op: OpPut, Key: "k1", Value: "v2"})
	if res.Err != nil {
		t.Fatalf("unexpected error on Update Put: %v", res.Err)
	}

	res = sm.Apply(Command{Op: OpGet, Key: "k1"})
	if res.Value != "v2" {
		t.Fatalf("expected value 'v2', got '%s'", res.Value)
	}

	// Test Delete
	res = sm.Apply(Command{Op: OpDelete, Key: "k1"})
	if res.Err != nil {
		t.Fatalf("unexpected error on Delete: %v", res.Err)
	}
	res = sm.Apply(Command{Op: OpGet, Key: "k1"})
	if res.Err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound after Delete, got %v", res.Err)
	}
}

// TestKVStateMachine_OutOfOrderDedup verifies idempotent replay by key
// and specifically allows skipping sequence numbers per PRD requirements.
func TestKVStateMachine_OutOfOrderDedup(t *testing.T) {
	sm := NewKVStateMachine()

	// Register a new client
	res := sm.Apply(Command{Op: OpRegister})
	if res.Err != nil {
		t.Fatalf("failed to register client: %v", res.Err)
	}
	clientID, _ := strconv.ParseInt(res.Value, 10, 64)

	// Apply SeqNum 1
	cmd1 := Command{Op: OpPut, Key: "a", Value: "1", ClientID: clientID, SeqNum: 1}
	if res := sm.Apply(cmd1); res.Err != nil {
		t.Fatalf("failed to apply seq 1: %v", res.Err)
	}

	// Apply SeqNum 3 (skipping 2)
	cmd3 := Command{Op: OpPut, Key: "a", Value: "3", ClientID: clientID, SeqNum: 3}
	if res := sm.Apply(cmd3); res.Err != nil {
		t.Fatalf("failed to apply seq 3: %v", res.Err)
	}

	// Verify state is '3'
	if sm.Apply(Command{Op: OpGet, Key: "a"}).Value != "3" {
		t.Fatalf("expected '3'")
	}

	// Retry SeqNum 1 (older than latest)
	// It should return success (cached result) without overwriting 'a'
	resDup1 := sm.Apply(cmd1)
	if resDup1.Err != nil {
		t.Fatalf("expected cached success for seq 1, got err: %v", resDup1.Err)
	}

	// State should STILL be '3'
	if sm.Apply(Command{Op: OpGet, Key: "a"}).Value != "3" {
		t.Fatalf("retry of seq 1 mutated state backwards!")
	}

	// Retry SeqNum 3
	resDup3 := sm.Apply(cmd3)
	if resDup3.Err != nil {
		t.Fatalf("expected cached success for seq 3, got err: %v", resDup3.Err)
	}
}

// TestKVStateMachine_CrashHarness verifies that a snapshot can be taken and restored
// into a brand new state machine, preserving exact state and ALL dedup sessions.
func TestKVStateMachine_CrashHarness(t *testing.T) {
	smA := NewKVStateMachine()

	// Register client
	resReg := smA.Apply(Command{Op: OpRegister})
	clientID, _ := strconv.ParseInt(resReg.Value, 10, 64)

	// Apply multiple commands to build up state
	commands := []Command{
		{Op: OpPut, Key: "user1", Value: "Alice", ClientID: clientID, SeqNum: 1},
		{Op: OpPut, Key: "user2", Value: "Bob", ClientID: clientID, SeqNum: 2},
		{Op: OpPut, Key: "user3", Value: "Charlie", ClientID: clientID, SeqNum: 3},
		{Op: OpPut, Key: "user1", Value: "Alice_Updated", ClientID: clientID, SeqNum: 4}, // Overwrite
	}

	for _, cmd := range commands {
		if res := smA.Apply(cmd); res.Err != nil {
			t.Fatalf("failed to apply %v: %v", cmd, res.Err)
		}
	}

	// Capture snapshot
	snap, err := smA.Snapshot()
	if err != nil {
		t.Fatalf("failed to take snapshot: %v", err)
	}

	// Simulate crash & restart with a new instance
	smB := NewKVStateMachine()

	// Restore from snapshot
	if err := smB.Restore(snap); err != nil {
		t.Fatalf("failed to restore snapshot: %v", err)
	}

	// Verify restored state
	tests := []struct {
		key      string
		expected string
	}{
		{"user1", "Alice_Updated"},
		{"user2", "Bob"},
		{"user3", "Charlie"},
	}

	for _, tc := range tests {
		res := smB.Apply(Command{Op: OpGet, Key: tc.key})
		if res.Err != nil {
			t.Fatalf("failed to get %s: %v", tc.key, res.Err)
		}
		if res.Value != tc.expected {
			t.Fatalf("expected %s for %s, got %s", tc.expected, tc.key, res.Value)
		}
	}

	// CRITICAL: Verify dedup table survived the crash for BOTH old and newest sequence numbers
	
	// Retrying sequence 1 should return its cached result without mutating
	resDedup1 := smB.Apply(Command{Op: OpPut, Key: "user1", Value: "Alice", ClientID: clientID, SeqNum: 1})
	if resDedup1.Err != nil {
		t.Fatalf("expected dedup for seq 1 to succeed, got err: %v", resDedup1.Err)
	}

	// Retrying sequence 4 should also return cached result
	resDedup4 := smB.Apply(Command{Op: OpPut, Key: "user1", Value: "Alice_Rollback", ClientID: clientID, SeqNum: 4})
	if resDedup4.Err != nil {
		t.Fatalf("expected dedup for seq 4 to succeed, got err: %v", resDedup4.Err)
	}
	
	resAfterDedup := smB.Apply(Command{Op: OpGet, Key: "user1"})
	if resAfterDedup.Value != "Alice_Updated" {
		t.Fatalf("dedup failed to prevent mutation across restart! expected 'Alice_Updated', got '%s'", resAfterDedup.Value)
	}
	
	// Verify Register counter survived
	resReg2 := smB.Apply(Command{Op: OpRegister})
	clientID2, _ := strconv.ParseInt(resReg2.Value, 10, 64)
	if clientID2 != 2 {
		t.Fatalf("expected next client ID to be 2, got %d", clientID2)
	}
}
