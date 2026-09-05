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
	// Because heartbeats are flowing, followers should have term >= leaderTerm
	for i, rf := range rafts {
		term, isLeader := rf.GetState()
		if !isLeader && term > leaderTerm {
			t.Fatalf("follower %d has term %d > leader term %d", i, term, leaderTerm)
		}
	}
}
