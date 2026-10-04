# Kibana Alert Rule & Webhook Connector Setup Guide

This guide provides step-by-step instructions for configuring Kibana Alert Rules and Webhook Connectors directly through the Kibana UI without relying on automated shell scripts.

---

## Part 1: Create Webhook Connector (Connect to AI RCA Agent)

1. Open your browser and navigate to **Kibana**: `http://localhost:5601`.
2. In the bottom-left navigation menu, click **Stack Management**.
3. Under **Alerts and Insights**, select **Connectors**.
4. Click the **Create connector** button (top right).
5. Choose **Webhook** as the connector type.
6. Fill in the following details:
   - **Name:** `RCA Agent Webhook`
   - **Method:** `POST`
   - **URL:** `http://rca-agent:8080/webhook/alert` *(or `http://localhost:8080/webhook/alert` if running locally outside Docker)*
   - **Authentication:** `None`
7. Click **Save & close**.

---

## Part 2: Create Alert Rule (Define Anomaly / Error Threshold)

1. In the **Stack Management** menu, select **Rules**.
2. Click **Create rule**.
3. Configure the general properties:
   - **Name:** `High 5xx Spike on App`
   - **Check every:** `1 minute`
   - **Notify:** `On check`
4. Define the detection condition (**Rule type**):
   - Select **Elasticsearch query**.
   - **Index:** `logs-app-*`
   - **Query (KQL):**
     ```text
     level: "ERROR" or http_status >= 500
     ```
   - **Threshold:** `IS ABOVE 3` in the last `1 minute`.  
     *(Trigger condition: fires when more than 3 ERROR or 5xx logs appear within 1 minute)*.
5. Configure the alert trigger (**Actions**):
   - Under **Actions**, select your created connector: `RCA Agent Webhook`.
   - Action group: **Query matched**.
   - In the **Body (JSON)** editor, provide the payload template:
     ```json
     {
       "rule_id": "{{{rule.id}}}",
       "rule_name": "{{{rule.name}}}",
       "timestamp": "{{{date}}}",
       "service": "order-service",
       "alert_reason": "High error rate detected in the last minute"
     }
     ```
6. Click **Save**.

---

## How It Works
- Kibana evaluates `logs-app-*` every minute.
- Once any fault scenario emits more than 3 errors within a 1-minute window, Kibana triggers an HTTP POST to the AI RCA Agent.
- Rules can be enabled, disabled, or tested directly from the Kibana UI at any time.
