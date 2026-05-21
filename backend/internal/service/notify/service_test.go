// Integration tests for the notify bridge. After the messages/
// notifications split, this package is ops-fanout only — the
// per-user lifecycle tests moved to service/messages. What stays
// here exercises the ops-event handlers (node.*, order.*) against
// a real Postgres dedup log; ops-test.go covers them with an
// in-memory stub for fast-feedback purposes.
//
// The only DB-backed concern remaining is making sure migrations
// still run when other packages depend on the schema; the
// integration tests for the ops path live in ops_test.go.
package notify
