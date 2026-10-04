# Runbook: PostgreSQL & HikariCP Connection Pool Exhaustion

## Metadata
- **Category:** infrastructure
- **Target Role:** sre
- **System:** order-service, postgresql

## Symptoms & Triage
- Error message: `HikariPool-1 - Connection is not available, request timed out after 30000ms`.
- Spike in 500 Internal Server Errors on checkout endpoints.

## Immediate Mitigation Steps
1. Drain traffic and restart `order-service` pods to immediately release stuck connections:
   `kubectl rollout restart deployment/order-service -n production`
2. Temporarily increase `maximum-pool-size` in ConfigMap from default `5` to `25`.
3. Check PostgreSQL active connections:
   `SELECT count(*) FROM pg_stat_activity WHERE datname = 'orderdb';`

## Permanent Infrastructure Remediation
- Deploy PgBouncer connection pooler in front of PostgreSQL to prevent server-side exhaustion.
- Enforce max lifetime on connections (`maxLifetime = 600000ms`).
