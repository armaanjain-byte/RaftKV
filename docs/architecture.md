# Architecture

Client -> KV service -> Raft -> State machine -> Persistent state

Raft <-> Transport interface
             |       |
            TCP     deterministic simulator -> fault injector
