# scheduler-jobs

The cron registry that owns every periodic backend job — node probing,
traffic collection, persistent webhook retry. Lives in `internal/job`.

## Purpose & boundaries

The `Scheduler` is a thin wrapper over `robfig/cron/v3` that adds:
- A `RunOnce` invocation path so tests can trigger jobs without
  waiting on the cron tick.
- A drain-aware `Stop()` returning a context that completes when
  in-flight job invocations finish — used by graceful shutdown in
  `internal/app/app.go`.

Job functions themselves live in `internal/job/{probe,traffic,webhook}.go`
and depend on the runtime / service-layer wiring assembled by
`internal/app/app.go::Build`.

## Requirements

### Requirement: Single Scheduler Per Process

The system SHALL construct exactly one `Scheduler` instance at startup
in `app.Build` and use it as the destination for every periodic job.

#### Scenario: Scheduler instantiated by app.Build

- **WHEN** `app.Build(cfg, db, logger)` runs
- **THEN** it SHALL call `job.NewScheduler(logger)` exactly once
- **AND** expose the scheduler on the returned `*App` so tests and the binary's `main.go` can reach it

### Requirement: Job Registration API

The system SHALL accept jobs via `Scheduler.Add(id, spec, fn)` where:
- `id` is a stable string used by `RunOnce(id)` to invoke the job
  outside the cron loop.
- `spec` is either a standard cron expression (5 fields) or an
  `@every <duration>` shorthand (e.g. `@every 30s`).
- `fn func(context.Context)` is invoked on every tick.

#### Scenario: Job with @every cadence

- **WHEN** the probe loop calls `Add("probe.nodes", "@every 30s", probeAll)`
- **THEN** the scheduler SHALL register the function with robfig/cron and store the (id, fn) pair internally
- **AND** every 30 seconds while `Start()` has been called, the function SHALL run with a fresh `context.Background()`

#### Scenario: Invalid spec rejected at registration

- **WHEN** `Add` receives a malformed cron spec
- **THEN** the call SHALL return a non-nil error from the underlying parser
- **AND** the job SHALL NOT be registered

### Requirement: RunOnce For Tests

The system SHALL invoke any registered job synchronously by id via
`RunOnce(id, ctx)` — independent of the cron clock.

#### Scenario: RunOnce a registered job

- **GIVEN** a job registered with id `"webhook.retry"`
- **WHEN** test code calls `scheduler.RunOnce("webhook.retry", ctx)`
- **THEN** the scheduler SHALL invoke the stored function with the supplied context
- **AND** the call SHALL block until the function returns

#### Scenario: RunOnce unknown id

- **WHEN** `RunOnce` is called with an id that was never `Add`-ed
- **THEN** the call SHALL be a no-op (logged at WARN level), returning without error
- **AND** SHALL NOT panic

### Requirement: Drain-Aware Shutdown

The system SHALL allow callers to wait for in-flight jobs to finish
during shutdown, so the process does not exit mid-iteration.

#### Scenario: Stop returns a drain context

- **WHEN** `scheduler.Stop()` is called
- **THEN** the scheduler SHALL stop accepting new ticks
- **AND** SHALL return a context that becomes `Done` when every currently-running job invocation has returned
- **AND** the caller (`app.Shutdown`) SHALL wait on that context, bounded by `Server.ShutdownTimeout`

### Requirement: Registered Jobs

The system SHALL ship with the following jobs registered by
`app.Build`:

#### Scenario: Probe job

- **WHEN** `app.Build` finishes wiring
- **THEN** a job with id `probe.nodes` and cadence `@every 30s` SHALL be registered
- **AND** the function SHALL call `nodeService.ProbeAll(ctx)` which probes every enabled node via `runtime.Manager.ForEach`

#### Scenario: Traffic snapshot job

- **WHEN** `app.Build` finishes wiring
- **THEN** a job with id `traffic.snapshot` and cadence `@every 60s` SHALL be registered
- **AND** it SHALL call into the traffic service to collect inbound/client snapshots and persist `traffic_samples` rows

#### Scenario: Webhook retry job

- **WHEN** `app.Build` finishes wiring
- **THEN** a job with id `webhook.retry` and cadence `@every 10s` SHALL be registered
- **AND** it SHALL pull `webhook_deliveries` rows where `status='pending' AND next_attempt_at <= now()` and re-issue them with exponential backoff
- **AND** the partial index `webhook_deliveries_due` from migration `0003_webhook_retry.up.sql` SHALL be used by the planner

## Out of scope

- Distributed scheduling across multiple dashboard replicas (assumed
  single-process today).
- Per-job pause/resume controls.
- Per-job execution history persisted to the DB (logs are the only
  history in v1).
