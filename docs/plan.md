# SimpleCache implementation plan

Focus: Go distributed-system reliability and Kubernetes platform engineering. Retain the current project and existing English documentation/CI. The current scope is a one-to-two-week functional and diagnostic pass.

## Progress

| Work | State |
| --- | --- |
| Module/package organization | Complete |
| Reliable peer/source reads and graceful lifecycle | Complete |
| EndpointSlice membership and atomic snapshots | Complete; local contracts verified |
| Root CI and English documentation | Complete; hosted runs remain separate |
| Complete Operator/deployment integration | Next |
| Monitoring, resource experiments, Profiling | Pending |

## Next: Operator and real cluster, about 2–3 days

Reconcile Services, StatefulSet, cache account and discovery permissions; watch children; add resources/probes/Status; reject unrelated conflicts; preserve immutable/defaulted fields. Remove static PEERS updates and revise their current test assertions.

Acceptance: envtest plus dedicated Kind startup/read; child repair; 3→5→2 membership convergence without restarting existing Pods just to publish peers. Retain app.serve, existing cancellation/boundary tests, and current CI.

## Then: monitoring and diagnosis, about 1–2 days

Replace fixed-Pod scrape targets, provision a small Grafana dashboard, and add missing capacity observations. Offer disabled-by-default admin Profiling.

Acceptance: all active Pods are visible; counters match actual operations; CPU/heap/mutex/block captures are reproducible. Identify a real bottleneck before choosing an optimization.

## Then: bounded experiments, about 2–3 days

| Scenario | Evidence |
| --- | --- |
| Cold hot key / slow peer | Correct values, actual source calls, fallback and elapsed time |
| Scale / owner Pod loss | Convergence, Pod UIDs/restarts, routing changes |
| Small cache | Logical byte bounds, eviction and successful reload |
| CPU/memory limits | Throttling/heap/Pod observations, errors and latency |
| Infeasible resource request | Pending/events, controller condition, recovery |

Use a fixed dataset/seed, explicit resources, warmup and value checks. Start with 10–30 second diagnostic samples. Record P50/P95/P99 only for actual runs; do not invent QPS or improvement percentages. Any controlled OOM belongs to one named Pod in the dedicated cluster, never host-wide pressure.

## Delivery, about 1 day

Add lightweight Kubernetes CI after local smoke is stable. Keep slow experiments manual. Publish one investigation: symptom → hypothesis → evidence/profile → change → measured comparison.

Complete means active code, repeatable cluster behavior, useful metrics, one resource/failure demonstration, and accurate limitations. Performance proof is separate from functional tests.

Do not add HPA against an Operator-owned StatefulSet without defining a single replica authority. Scheduler plugins, GPU management, and AI service orchestration are separate later specialization, not cache feature additions.
