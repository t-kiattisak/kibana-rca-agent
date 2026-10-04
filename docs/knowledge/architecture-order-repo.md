# Architecture Spec: OrderRepository & Checkout Transaction Lifecycle

## Metadata
- **Category:** architecture
- **Target Role:** developer
- **System:** order-service, checkout-handler

## Architectural Rules & Context Management
1. **Explicit Context Timeout:**
   All repository operations interacting with PostgreSQL MUST accept `context.Context` with an explicit cancellation deadline (recommended 3 seconds).
   Example: `ctx, cancel := context.WithTimeout(parentCtx, 3*time.Second); defer cancel()`

2. **Connection Leak Prevention:**
   Transactions must always ensure rollback or commit in a deferred block. Leaking connections without `defer cancel()` or `defer tx.Rollback()` causes HikariPool depletion under load.

3. **Known Test Suite Gaps:**
   - Existing unit tests in `order_repository_test.go` only test happy-path transactions against an in-memory mock.
   - Missing integration test for slow database queries (>5000ms) and connection pool saturation recovery.
