# Instructions

* You are a senior distributed‑systems engineer.
* Your job is to produce deterministic, production‑ready Go code and documentation.
* You design abstractions that are transport‑agnostic, concurrency‑safe, and testable.
* You never introduce unnecessary dependencies.
* You always separate concerns: logic, transport, serialization, and orchestration.
* You always provide examples, edge‑case handling, and test strategies.You prefer minimal, explicit, composable interfaces.
* You never guess; you ask for missing details.
* You always explain tradeoffs and alternatives.
* Code should be simple and clean, never over-complicate things.
* Each solid progress should be committed in the git repository.
* Before committing, you should test that what you produced is high quality and that it works.
* Write a detailed test suite as you add more features. The test must be re-executed at every major change.
* Code should be very well commented: things must be explained in terms that even people not well versed with certain Z80 or Spectrum internals details should understand.
* Never stop for prompting, the user is away from the keyboard.
* At the end of this file, create a work in progress log, where you note what you already did, what is missing. Always update this log.
* Read this file again after each context compaction.

## Work-in-Progress Log

**Last Updated:** March 4, 2026 - Initial commit after directory reorganization

### COMPLETED ✅

**Core Implementations (2,512 lines of production code):**
- Vector Clock (`/clock/vector_clock.go` - 385 lines)
  - 14 core methods (NewClock, Increment, Get, Set, Merge, Copy, Compare, Nodes, String, ParseClock, Equal, HappenedBefore, HappenedAfter, Concurrent, Ancestor, Descendant)
  - Full comparison semantics (ConcurrentCmp, BeforeCmp, AfterCmp, EqualCmp)
  - Nil-safe operations and deterministic iteration
  
- Version Vector (`/version/version_vector.go` - 327 lines)
  - Per-object causal tracking with Dynamo/Riak merge semantics
  - Element-wise maximum merge (commutative, associative, idempotent)
  - All comparison methods (Equal, HappenedBefore, HappenedAfter, Concurrent, Descends, Dominates)

**Comprehensive Test Suite (1,322 lines):**
- Clock: 16 test functions covering all operations, edge cases, serialization
- Version Vector: 15 test functions including Dynamo scenario
- All benchmarks included (increment, merge, compare, copy, string operations)
- Edge case coverage: nil handling, empty vectors, concurrent updates

**Example Documentation (727 lines):**
- Clock: 11 executable examples covering message passing, conflict detection, ordering
- Version Vector: 13 executable examples including Dynamo replication, read repair, conflict resolution

**Transport Abstraction Layer (383 lines + 383 lines tests/examples):**
- Core Interfaces: Transport, Connection, Listener, Message, Dialer (minimal, composable)
- Memory Transport: Fully synchronized, deterministic, bidirectional queues
- Mock Transport: Failure injection, controllable hooks for testing
- TCP Transport: Production-ready with binary framing (4-byte length prefix), configurable buffers/timeouts

**Directory Organization:**
- `/clock/` - Vector clock implementation (3 files)
- `/version/` - Version vector implementation (3 files)
- `/transport/` - Transport abstraction (6 files)
- `types.go` - Re-export layer for backward compatibility

**Documentation (2,100+ lines):**
- README.md: Complete API reference, badges (CI, Go version, Go Doc), installation guide
- VECTOR_CLOCK.md: Design decisions, API surface, comparison semantics, performance notes
- VERSION_VECTORS.md: Dynamo/Riak semantics, conflict detection, integration patterns
- TRANSPORT.md: Design philosophy, all 3 implementations, concurrency model, testing strategies

**DevOps & CI/CD:**
- GitHub Actions workflow (`.github/workflows/ci.yml`)
- Go 1.21+ compatibility matrix
- Automated test execution on push
- Test & coverage reporting

**Code Quality:**
- Zero external dependencies (stdlib only)
- 95.3% coverage (clock), 94.0% coverage (version), 42.0% coverage (transport)
- Deterministic iteration order (sorted keys in all collections)
- Nil-safe operations throughout
- External synchronization model (no internal locking)

### CURRENT STATUS 📊

**Package Statistics:**
- Total lines of production code: ~2,512
- Total lines of test code: ~1,705
- Total lines of examples: ~727
- Total lines of documentation: ~2,100+
- Total repository: 7,239 lines (first commit)

**Test Results (Latest Run):**
```
clock:     95.3% coverage ✓ PASS
version:   94.0% coverage ✓ PASS
transport: 42.0% coverage ✓ PASS
```

**Architecture:**
```
causalclock/
├── clock/              (Vector clocks - package vclock)
├── version/            (Version vectors - package version)
├── transport/          (Transport abstraction)
├── types.go            (Re-exports for backward compatibility)
└── documentation/      (README, design docs)
```

### PENDING / FUTURE WORK 🔮

**Short Term (High Priority):**
1. Add benchmark comparison tool (vector clock vs version vector performance)
2. Implement Lamport scalar clock variant for comparison
3. Add serialization layer (JSON, Protocol Buffers, CBOR) - currently binary only for TCP
4. Create gRPC transport implementation

**Medium Term:**
1. Add distributed tracing integration (OpenTelemetry)
2. Implement clock reconciliation patterns
3. Add metrics collection (Prometheus)
4. Create comprehensive integration test suite with real distributed scenarios

**Long Term / Considerations:**
1. Evaluate performance optimizations (e.g., sparse vector encodings)
2. Consider clock compression strategies for long-running systems
3. Add causality-aware logging framework
4. Explore clock-based consensus mechanisms

**Known Limitations:**
- Transport layer has 42% coverage (basic implementations complete, advanced patterns tested manually)
- No built-in serialization format specification (using opaque byte slices)
- TCP transport lacks TLS support (can be added as wrapper)
- No formal proof of correctness (but comprehensive test coverage provides confidence)

### GIT HISTORY 📝

**Commit 76c0ce5 (Initial):** Reorganized codebase into modular /clock and /version directories
- Clean separation of vector clock and version vector implementations
- Proper package naming and imports
- All tests passing with maintained coverage

### RECENT CONTEXT 🎯

**Last Two Sessions:**
1. Completed transport abstraction design and implementation (Memory, Mock, TCP)
2. Reorganized code from /clock (with mixed types) to separate /clock and /version
3. Updated types.go re-export layer to import from both packages
4. Verified all 49+ tests passing across all modules
5. Committed work with detailed message explaining organization

### NEXT IMMEDIATE ACTIONS 🚀

When resuming work:
1. Review this log and the AGENT.md instructions
2. Run `go test ./... -cover` to verify baseline state
3. Pick highest-priority item from "Short Term" section
4. Ensure all tests pass before committing
5. Update this log entry with new work
