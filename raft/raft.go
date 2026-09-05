package raft

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type State int

const (
	StateFollower State = iota
	StateCandidate
	StateLeader
)

// NetworkPeer defines how a Raft node communicates with another node.
type NetworkPeer interface {
	RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) bool
	AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) bool
}

type Raft struct {
	mu        sync.Mutex
	peers     []NetworkPeer
	persister *Persister
	me        int
	dead      int32

	state       State
	currentTerm int
	votedFor    int

	electionTime time.Time
}

type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	// Entries []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

func Make(peers []NetworkPeer, me int, persister *Persister) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.state = StateFollower
	rf.votedFor = -1

	// read from crash
	rf.readPersist(persister.ReadRaftState())

	rf.resetElectionTimer()
	
	go rf.ticker()
	
	return rf
}

func (rf *Raft) persist() {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(rf.currentTerm)
	enc.Encode(rf.votedFor)
	rf.persister.SaveRaftState(buf.Bytes())
}

func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 {
		return
	}
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var term int
	var votedFor int
	if err := dec.Decode(&term); err == nil {
		rf.currentTerm = term
	}
	if err := dec.Decode(&votedFor); err == nil {
		rf.votedFor = votedFor
	}
}

func (rf *Raft) resetElectionTimer() {
	// randomized timeout between 300ms and 600ms
	timeout := time.Duration(300+rand.Intn(300)) * time.Millisecond
	rf.electionTime = time.Now().Add(timeout)
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// PRD Requirement: Exact two-branch form
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = -1
		rf.state = StateFollower
		rf.persist()
	}

	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		// Log up-to-date check goes here (PR 3). For PR 1, we grant it unconditionally if votedFor matches.
		rf.votedFor = args.CandidateId
		rf.state = StateFollower
		rf.persist()
		rf.resetElectionTimer()
		reply.VoteGranted = true
	}
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// PRD Requirement: Exact two-branch form
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = -1
		rf.state = StateFollower
		rf.persist()
	}

	reply.Term = rf.currentTerm
	reply.Success = true

	// If the leader's term is >= ours, we recognize them.
	// We reset the election timer on a VALID AppendEntries from the CURRENT leader.
	rf.state = StateFollower
	rf.resetElectionTimer()
}

func (rf *Raft) ticker() {
	for !rf.killed() {
		time.Sleep(10 * time.Millisecond)
		rf.mu.Lock()
		if rf.state != StateLeader && time.Now().After(rf.electionTime) {
			rf.startElection()
		}
		rf.mu.Unlock()
	}
}

func (rf *Raft) startElection() {
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.state = StateCandidate
	rf.persist()
	rf.resetElectionTimer()

	term := rf.currentTerm
	me := rf.me
	
	args := &RequestVoteArgs{
		Term:         term,
		CandidateId:  me,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	votes := 1

	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(peer int) {
			reply := &RequestVoteReply{}
			ok := rf.peers[peer].RequestVote(args, reply)
			if !ok {
				return
			}
			
			rf.mu.Lock()
			defer rf.mu.Unlock()
			
			if rf.state != StateCandidate || rf.currentTerm != term {
				return
			}
			
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.votedFor = -1
				rf.state = StateFollower
				rf.persist()
				return
			}
			
			if reply.VoteGranted {
				votes++
				if votes > len(rf.peers)/2 {
					rf.becomeLeader()
				}
			}
		}(i)
	}
}

func (rf *Raft) becomeLeader() {
	rf.state = StateLeader
	// Start heartbeating
	go rf.heartbeatLoop()
}

func (rf *Raft) heartbeatLoop() {
	for !rf.killed() {
		rf.mu.Lock()
		if rf.state != StateLeader {
			rf.mu.Unlock()
			return
		}
		term := rf.currentTerm
		me := rf.me
		rf.mu.Unlock()

		args := &AppendEntriesArgs{
			Term:     term,
			LeaderId: me,
		}

		for i := range rf.peers {
			if i == me {
				continue
			}
			go func(peer int) {
				reply := &AppendEntriesReply{}
				rf.peers[peer].AppendEntries(args, reply)
				// Basic handling for PR 1
				rf.mu.Lock()
				defer rf.mu.Unlock()
				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.votedFor = -1
					rf.state = StateFollower
					rf.persist()
				}
			}(i)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// GetState returns currentTerm and whether this server believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.state == StateLeader
}
