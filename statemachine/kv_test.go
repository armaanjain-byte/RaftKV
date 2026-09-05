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

func TestKVStateMachine_DedupAndRegister(t *testing.T) {
	sm := NewKVStateMachine()

	// Register a new client
	res := sm.Apply(Command{Op: OpRegister})
	if res.Err != nil {
		t.Fatalf("failed to register client: %v", res.Err)
	}
	clientID, err := strconv.ParseInt(res.Value, 10, 64)
	if err != nil || clientID != 1 {
		t.Fatalf("expected ClientID 1, got %v (err: %v)", res.Value, err)
	}

	// Apply a command with a sequence number
	cmd := Command{Op: OpPut, Key: "a", Value: "1", ClientID: clientID, SeqNum: 1}
	res = sm.Apply(cmd)
	if res.Err != nil {
		t.Fatalf("failed to apply command: %v", res.Err)
	}

	// Apply the EXACT SAME command (duplicate)
	resDup := sm.Apply(cmd)
	if resDup.Err != nil || resDup.Value != res.Value {
		t.Fatalf("deduplication failed, expected %v, got %v", res, resDup)
	}

	// Overwrite key 'a' without using ClientID (simulate external mutation for test purpose)
	sm.Apply(Command{Op: OpPut, Key: "a", Value: "2"})

	// Apply the duplicate command AGAIN, it should STILL return the original result,
	// and critically, it should NOT overwrite 'a' back to '1'.
	resDup2 := sm.Apply(cmd)
	if resDup2.Err != nil || resDup2.Value != res.Value {
		t.Fatalf("deduplication failed on second attempt, expected %v, got %v", res, resDup2)
	}

	resCurrent := sm.Apply(Command{Op: OpGet, Key: "a"})
	if resCurrent.Value != "2" {
		t.Fatalf("deduplication incorrectly mutated state! expected '2', got '%s'", resCurrent.Value)
	}

	// Apply an older sequence number
	resStale := sm.Apply(Command{Op: OpPut, Key: "a", Value: "3", ClientID: clientID, SeqNum: 0})
	if resStale.Err != ErrStaleCommand {
		t.Fatalf("expected ErrStaleCommand, got %v", resStale.Err)
	}
}

// TestKVStateMachine_CrashHarness verifies that a snapshot can be taken and restored
// into a brand new state machine, preserving exact state and dedup sessions.
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

	// Ensure new instance is empty
	if res := smB.Apply(Command{Op: OpGet, Key: "user1"}); res.Err != ErrKeyNotFound {
		t.Fatalf("expected new state machine to be empty")
	}

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

	// CRITICAL: Verify dedup table survived the crash
	// Applying sequence 4 again should return the deduped result, not modify anything, and not error
	resDedup := smB.Apply(Command{Op: OpPut, Key: "user1", Value: "Alice_Rollback", ClientID: clientID, SeqNum: 4})
	if resDedup.Err != nil {
		t.Fatalf("expected dedup to succeed, got err: %v", resDedup.Err)
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
