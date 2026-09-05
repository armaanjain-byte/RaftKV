package raft

import (
	"sync"
	"time"
)

// LocalNetwork simulates a local in-memory network for testing.
type LocalNetwork struct {
	mu    sync.Mutex
	rafts map[int]*Raft
}

func MakeNetwork() *LocalNetwork {
	return &LocalNetwork{
		rafts: make(map[int]*Raft),
	}
}

func (n *LocalNetwork) AddNode(i int, rf *Raft) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.rafts[i] = rf
}

type LocalPeer struct {
	targetId int
	network  *LocalNetwork
}

func (p *LocalPeer) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) bool {
	p.network.mu.Lock()
	target, ok := p.network.rafts[p.targetId]
	p.network.mu.Unlock()

	if !ok || target.killed() {
		return false
	}
	
	// Simulate network delay and prevent synchronous deadlock
	time.Sleep(2 * time.Millisecond)
	target.RequestVote(args, reply)
	return true
}

func (p *LocalPeer) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	p.network.mu.Lock()
	target, ok := p.network.rafts[p.targetId]
	p.network.mu.Unlock()

	if !ok || target.killed() {
		return false
	}
	
	time.Sleep(2 * time.Millisecond)
	target.AppendEntries(args, reply)
	return true
}

func SetupCluster(num int) ([]*Raft, *LocalNetwork) {
	net := MakeNetwork()
	rafts := make([]*Raft, num)
	
	for i := 0; i < num; i++ {
		peers := make([]NetworkPeer, num)
		for j := 0; j < num; j++ {
			peers[j] = &LocalPeer{targetId: j, network: net}
		}
		persister := MakePersister()
		rafts[i] = Make(peers, i, persister)
		net.AddNode(i, rafts[i])
	}
	
	return rafts, net
}

func CleanupCluster(rafts []*Raft) {
	for _, rf := range rafts {
		if rf != nil {
			rf.Kill()
		}
	}
}
