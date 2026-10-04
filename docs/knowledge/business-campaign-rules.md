# Business Policy: Checkout SLA, Campaign Continuity & Degraded Mode

## Metadata
- **Category:** business_policy
- **Target Role:** product
- **System:** order-service, payment-gateway

## Service Level Objectives (SLO) & Business Impact
- Maximum tolerable downtime during marketing campaigns: **3 minutes**.
- Each 1 minute of complete checkout failure impacts an estimated **$15,000 USD in Gross Merchandise Value (GMV)**.

## Degraded Mode Policy (ADR-005 Compliance)
- If primary payment gateway or synchronous database checkout fails for > 60 seconds:
  1. DO NOT completely block user checkout.
  2. Switch client frontend to **"Asynchronous Order Intake Mode"**: Accept orders into Kafka queue with status `PENDING_CONFIRMATION` and display "Your order is being processed" to prevent cart abandonment.
  3. Notify marketing operations team if campaign orders are delayed by more than 15 minutes.
