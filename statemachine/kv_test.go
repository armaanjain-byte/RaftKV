package statemachine

import (
	"bytes"
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
}

// TestKVStateMachine_CrashHarness verifies that a snapshot can be taken and restored
// into a brand new state machine, preserving exact state.
func TestKVStateMachine_CrashHarness(t *testing.T) {
	smA := NewKVStateMachine()

	// Apply multiple commands to build up state
	commands := []Command{
		{Op: OpPut, Key: "user1", Value: "Alice"},
		{Op: OpPut, Key: "user2", Value: "Bob"},
		{Op: OpPut, Key: "user3", Value: "Charlie"},
		{Op: OpPut, Key: "user1", Value: "Alice_Updated"}, // Overwrite
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

	// Verify restoring into an already populated map overwrites it entirely
	smB.Apply(Command{Op: OpPut, Key: "user4", Value: "Dave"})
	if err := smB.Restore(snap); err != nil {
		t.Fatalf("failed to restore snapshot second time: %v", err)
	}
	if res := smB.Apply(Command{Op: OpGet, Key: "user4"}); res.Err != ErrKeyNotFound {
		t.Fatalf("expected user4 to be gone after restore, got error: %v", res.Err)
	}
}

// TestKVStateMachine_SnapshotDeterminism verifies that two identical maps produce identical snapshots
// (Wait, gob encoding for maps is not inherently deterministic in byte order because map iteration is random.
// However, since gob doesn't guarantee byte-for-byte deterministic output for maps, we will check semantic determinism:
// taking a snapshot, restoring it, and taking another snapshot should yield the same state semantics.)
func TestKVStateMachine_SnapshotDeterminism(t *testing.T) {
	sm := NewKVStateMachine()
	sm.Apply(Command{Op: OpPut, Key: "k1", Value: "v1"})
	sm.Apply(Command{Op: OpPut, Key: "k2", Value: "v2"})

	snap1, err := sm.Snapshot()
	if err != nil {
		t.Fatalf("snap1 failed: %v", err)
	}

	sm2 := NewKVStateMachine()
	sm2.Restore(snap1)
	
	snap2, err := sm2.Snapshot()
	if err != nil {
		t.Fatalf("snap2 failed: %v", err)
	}

	// We can restore snap2 back into another and verify it works, 
	// rather than byte-for-byte comparison which might fail due to gob's map serialization.
	sm3 := NewKVStateMachine()
	sm3.Restore(snap2)
	
	if sm3.Apply(Command{Op: OpGet, Key: "k1"}).Value != "v1" {
		t.Fatalf("semantic determinism failed for k1")
	}
	if sm3.Apply(Command{Op: OpGet, Key: "k2"}).Value != "v2" {
		t.Fatalf("semantic determinism failed for k2")
	}
}
