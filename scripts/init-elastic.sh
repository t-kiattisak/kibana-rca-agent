#!/usr/bin/env bash
set -e

ES_URL="${ELASTICSEARCH_URL:-http://localhost:9200}"
KIBANA_URL="${KIBANA_URL:-http://localhost:5601}"
INDEX_NAME="logs-app-dev"
VECTOR_INDEX="incident-memory-knn"

echo "⏳ 1. Waiting for Elasticsearch at $ES_URL..."
until curl -s "$ES_URL/_cluster/health" | grep -q '"status"'; do
  sleep 2
done
echo "✅ Elasticsearch is ready."

echo "📦 2. Creating Index Template / Mapping for $INDEX_NAME..."
curl -s -X PUT "$ES_URL/$INDEX_NAME" \
  -H "Content-Type: application/json" \
  -d '{
    "mappings": {
      "properties": {
        "@timestamp": { "type": "date" },
        "service": { "type": "keyword" },
        "trace_id": { "type": "keyword" },
        "level": { "type": "keyword" },
        "message": { "type": "text" },
        "http_method": { "type": "keyword" },
        "http_path": { "type": "keyword" },
        "http_status": { "type": "integer" },
        "duration_ms": { "type": "long" },
        "stack_trace": { "type": "text" }
      }
    }
  }' > /dev/null
echo "✅ Log Index created."

echo "🧠 3. Creating Vector Memory Index for $VECTOR_INDEX..."
curl -s -X PUT "$ES_URL/$VECTOR_INDEX" \
  -H "Content-Type: application/json" \
  -d '{
    "mappings": {
      "properties": {
        "incident_id": { "type": "keyword" },
        "service": { "type": "keyword" },
        "error_signature": { "type": "text" },
        "embedding": {
          "type": "dense_vector",
          "dims": 768,
          "index": true,
          "similarity": "cosine"
        },
        "root_cause_summary": { "type": "text" },
        "resolution_steps": { "type": "text" },
        "resolved_at": { "type": "date" }
      }
    }
  }' > /dev/null
echo "✅ Vector Index created."

echo "⏳ 4. Waiting for Kibana at $KIBANA_URL..."
until curl -s "$KIBANA_URL/api/status" | grep -q '"overall":{"level":"available"'; do
  sleep 3
done
echo "✅ Kibana is ready."

echo "🔌 5. Provisioning Kibana Webhook Connector to RCA Agent..."
CONNECTOR_RES=$(curl -s -X POST "$KIBANA_URL/api/actions/connector" \
  -H "kbn-xsrf: true" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "RCA Agent Webhook",
    "connector_type_id": ".webhook",
    "config": {
      "method": "POST",
      "url": "http://rca-agent:8080/webhook/alert",
      "hasAuth": false
    }
  }')
echo "✅ Webhook Connector response: $CONNECTOR_RES"

echo "🎉 Initialization completed successfully!"
