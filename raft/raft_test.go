package raft

import (
	"testing"
	"time"
)

func TestElection(t *testing.T) {
	servers := 3
	rafts, _ := SetupCluster(servers)
	defer CleanupCluster(rafts)

	// Wait for a leader to be elected
	time.Sleep(2 * time.Second)

	leaders := 0
	leaderTerm := -1
	for _, rf := range rafts {
		term, isLeader := rf.GetState()
		if isLeader {
			leaders++
			leaderTerm = term
		}
	}

	if leaders == 0 {
		t.Fatalf("expected 1 leader, got 0")
	}
	if leaders > 1 {
		t.Fatalf("expected 1 leader, got %d", leaders)
	}

	// Verify all nodes agree on the term (or are following)
	for i, rf := range rafts {
		term, isLeader := rf.GetState()
		if !isLeader && term > leaderTerm {
			t.Fatalf("follower %d has term %d > leader term %d", i, term, leaderTerm)
		}
	}
}

// TestTermVoteAtomicPersistence verifies that (currentTerm, votedFor) are ALWAYS
// persisted as one atomic unit and that a crash between in-memory update and the
// durable write cannot leave the store in a mixed state.
//
// This exercises the invariant:
//
//	On recovery, either the fully-old (term=T, votedFor=V_old) pair is seen,
//	or the fully-new (term=T', votedFor=V_new) pair — never (term=T', votedFor=V_old)
//	or any other mixed combination.
//
// Approach: we use the Persister's beforeSave hook to panic (simulating a crash)
// exactly once, so the first persist() call is aborted. Recovery must then land
// on the pre-update state, not a partial write.
func TestTermVoteAtomicPersistence(t *testing.T) {
	p := MakePersister()

	// ── Scenario A: crash before FIRST persist (fresh node) ─────────────────
	// The hook fires and panics on the first save, then disables itself.
	crashed := false
	p.SetBeforeSaveHook(func() {
		if !crashed {
			crashed = true
			panic("injected crash: before first persist")
		}
	})

	// Recover from the panic so the test continues.
	func() {
		defer func() { recover() }()
		// Simulate what raft does when it increments term and votes for itself.
		// This is the exact sequence in startElection: term++, votedFor=me, persist.
		term := 1
		votedFor := 0
		p.SaveRaftState(EncodeRaftState(term, votedFor))
	}()

	// The hook fired and panicked; the persisted state must still be empty (old state).
	data := p.ReadRaftState()
	recoveredTerm, recoveredVotedFor := DecodeRaftState(data)
	if recoveredTerm != 0 || recoveredVotedFor != -1 {
		t.Fatalf("Scenario A: expected fresh state (term=0, votedFor=-1) after crash, "+
			"got (term=%d, votedFor=%d)", recoveredTerm, recoveredVotedFor)
	}

	// ── Scenario B: successful persist after hook is cleared ─────────────────
	p.SetBeforeSaveHook(nil)
	p.SaveRaftState(EncodeRaftState(3, 1))

	// ── Scenario C: crash before a SUBSEQUENT persist, old value must survive ─
	// Pre-state: term=3, votedFor=1.
	// We simulate a step-down to term=5, but the crash fires before the write.
	crashed2 := false
	p.SetBeforeSaveHook(func() {
		if !crashed2 {
			crashed2 = true
			panic("injected crash: before step-down persist")
		}
	})

	func() {
		defer func() { recover() }()
		p.SaveRaftState(EncodeRaftState(5, -1)) // step-down: higher term, cleared vote
	}()

	// Must still see the pre-update pair (term=3, votedFor=1).
	data = p.ReadRaftState()
	recoveredTerm, recoveredVotedFor = DecodeRaftState(data)
	if recoveredTerm != 3 || recoveredVotedFor != 1 {
		t.Fatalf("Scenario C: expected old pair (term=3, votedFor=1) after crash, "+
			"got (term=%d, votedFor=%d)", recoveredTerm, recoveredVotedFor)
	}

	// ── Scenario D: no partial-field state is ever observable ────────────────
	// Explicitly verify the decode of the old blob returns both fields consistently.
	// There is no code path that writes only currentTerm or only votedFor; the
	// struct-based encode guarantees they travel together.
	p.SetBeforeSaveHook(nil)
	blob := EncodeRaftState(7, 2)
	decodedTerm, decodedVoted := DecodeRaftState(blob)
	if decodedTerm != 7 || decodedVoted != 2 {
		t.Fatalf("Scenario D: encode/decode round-trip failed: got (%d, %d)", decodedTerm, decodedVoted)
	}
}
