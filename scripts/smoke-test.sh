#!/bin/bash
set -e

BASE_URL="http://localhost:8000/api/v1"
DIRECT_URL="http://localhost:8080/api/v1"

echo "============================================"
echo "  Insider One Notification - Test Scenarios"
echo "============================================"
echo ""

# 1. Health Check
echo "--- 1. Health Check ---"
curl -s "$BASE_URL/health" | python3 -m json.tool
echo ""
sleep 1

# 2. Create SMS Notification
echo "--- 2. Create SMS Notification (high priority) ---"
SMS_RESPONSE=$(curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "+905551234567", "channel": "sms", "content": "Your verification code is 123456", "priority": "high"}')
echo "$SMS_RESPONSE" | python3 -m json.tool
SMS_ID=$(echo "$SMS_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "SMS ID: $SMS_ID"
echo ""
sleep 2

# 3. Create Email Notification
echo "--- 3. Create Email Notification (normal priority) ---"
EMAIL_RESPONSE=$(curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "user@example.com", "channel": "email", "content": "Welcome to Insider One!", "priority": "normal"}')
echo "$EMAIL_RESPONSE" | python3 -m json.tool
EMAIL_ID=$(echo "$EMAIL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo ""
sleep 2

# 4. Create Push Notification
echo "--- 4. Create Push Notification (low priority) ---"
PUSH_RESPONSE=$(curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "device-token-abc123", "channel": "push", "content": "You have a new message", "priority": "low"}')
echo "$PUSH_RESPONSE" | python3 -m json.tool
echo ""
sleep 2

# 5. Query by ID (should be delivered by now)
echo "--- 5. Query Notification by ID ---"
curl -s "$BASE_URL/notifications/$SMS_ID" | python3 -m json.tool
echo ""
sleep 1

# 6. Batch Creation
echo "--- 6. Batch Creation (3 notifications) ---"
BATCH_RESPONSE=$(curl -s -X POST "$BASE_URL/notifications/batch" \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {"recipient": "+905551111111", "channel": "sms", "content": "Flash sale starts now!", "priority": "high"},
      {"recipient": "admin@example.com", "channel": "email", "content": "Monthly report ready", "priority": "normal"},
      {"recipient": "device-token-xyz", "channel": "push", "content": "New feature available", "priority": "low"}
    ]
  }')
echo "$BATCH_RESPONSE" | python3 -m json.tool
BATCH_ID=$(echo "$BATCH_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['batch_id'])")
echo "Batch ID: $BATCH_ID"
echo ""
sleep 2

# 7. Query by Batch ID
echo "--- 7. Query by Batch ID ---"
curl -s "$BASE_URL/notifications/batch/$BATCH_ID" | python3 -m json.tool
echo ""
sleep 1

# 8. Template System
echo "--- 8. Template System ---"
TEMPLATE_RESPONSE=$(curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905551234567",
    "channel": "sms",
    "content": "Hi {{name}}, your order {{order_id}} is confirmed!",
    "priority": "high",
    "payload": {
      "template_vars": {
        "name": "Kubilay",
        "order_id": "ORD-98765"
      }
    }
  }')
echo "$TEMPLATE_RESPONSE" | python3 -m json.tool
TEMPLATE_ID=$(echo "$TEMPLATE_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo ""
sleep 2

# 9. Verify template was resolved
echo "--- 9. Verify Template Resolution ---"
curl -s "$BASE_URL/notifications/$TEMPLATE_ID" | python3 -m json.tool
echo ""
sleep 1

# 10. Idempotency
echo "--- 10. Idempotency Test (same key twice) ---"
curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "+905559999999", "channel": "sms", "content": "Idempotent message", "priority": "normal", "idempotency_key": "unique-key-123"}' | python3 -m json.tool
echo ""
echo "Sending same idempotency key again..."
curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "+905559999999", "channel": "sms", "content": "Idempotent message", "priority": "normal", "idempotency_key": "unique-key-123"}' | python3 -m json.tool
echo ""
sleep 1

# 11. List with Filters
echo "--- 11. List Notifications (filter: sms, page 1) ---"
curl -s "$BASE_URL/notifications?channel=sms&page=1&page_size=5" | python3 -m json.tool
echo ""
sleep 1

echo "--- 12. List Notifications (filter: delivered) ---"
curl -s "$BASE_URL/notifications?status=delivered&page=1&page_size=5" | python3 -m json.tool
echo ""
sleep 1

# 13. Cancel (create a new one, then cancel)
echo "--- 13. Cancel Notification ---"
CANCEL_RESPONSE=$(curl -s -X POST "$DIRECT_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "+905550000000", "channel": "sms", "content": "This will be cancelled", "priority": "low"}')
echo "Created: $CANCEL_RESPONSE" | python3 -m json.tool
CANCEL_ID=$(echo "$CANCEL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo ""
echo "Cancelling $CANCEL_ID..."
curl -s -X DELETE "$BASE_URL/notifications/$CANCEL_ID" -w "HTTP Status: %{http_code}\n"
echo ""
echo "Verify cancelled:"
curl -s "$BASE_URL/notifications/$CANCEL_ID" | python3 -m json.tool
echo ""
sleep 1

# 14. Validation Errors
echo "--- 14. Validation: Invalid Channel ---"
curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "+905551234567", "channel": "fax", "content": "test", "priority": "high"}' | python3 -m json.tool
echo ""

echo "--- 15. Validation: Empty Recipient ---"
curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "", "channel": "sms", "content": "test", "priority": "high"}' | python3 -m json.tool
echo ""

echo "--- 16. Validation: Empty Content ---"
curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "+905551234567", "channel": "sms", "content": "", "priority": "high"}' | python3 -m json.tool
echo ""

# 17. Scheduled Notification
echo "--- 17. Scheduled Notification (future) ---"
FUTURE_TIME=$(date -u -v+1H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "+1 hour" +"%Y-%m-%dT%H:%M:%SZ")
curl -s -X POST "$BASE_URL/notifications" \
  -H "Content-Type: application/json" \
  -d "{\"recipient\": \"+905551234567\", \"channel\": \"sms\", \"content\": \"Scheduled message\", \"priority\": \"normal\", \"scheduled_at\": \"$FUTURE_TIME\"}" | python3 -m json.tool
echo ""

# 18. WebSocket Real-time Updates
echo "--- 18. WebSocket Real-time Status Updates ---"
echo "Creating notification and listening for status changes via WebSocket..."

WS_RESPONSE=$(curl -s -X POST "$DIRECT_URL/notifications" \
  -H "Content-Type: application/json" \
  -d '{"recipient": "+905557777777", "channel": "sms", "content": "WebSocket test", "priority": "high"}')
WS_ID=$(echo "$WS_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Created notification: $WS_ID"
echo "Connecting to ws://localhost:8080/api/v1/ws/notifications/$WS_ID ..."

python3 -c "
import asyncio, json, websockets, sys

async def listen():
    uri = 'ws://localhost:8080/api/v1/ws/notifications/$WS_ID'
    try:
        async with websockets.connect(uri) as ws:
            print('Connected! Waiting for status updates (5s timeout)...')
            try:
                msg = await asyncio.wait_for(ws.recv(), timeout=5)
                data = json.loads(msg)
                print(json.dumps(data, indent=2))
            except asyncio.TimeoutError:
                print('No update received (notification may have been delivered before connection)')
    except Exception as e:
        print(f'WebSocket not available: {e}')
        print('Install: pip3 install websockets')

asyncio.run(listen())
" 2>/dev/null || echo "Skipped (install websockets: pip3 install websockets)"
echo ""
echo "Manual WebSocket test:"
echo "  Terminal 1: wscat -c ws://localhost:8080/api/v1/ws/notifications/{id}"
echo "  Terminal 2: curl -X POST http://localhost:8000/api/v1/notifications ..."
echo ""

# Summary
echo "============================================"
echo "  Test Complete! (18 scenarios)"
echo "============================================"
echo ""
echo "Check these services for results:"
echo "  Webhook deliveries : https://webhook.site (your URL)"
echo "  Jaeger traces      : http://localhost:16686"
echo "  Prometheus metrics : http://localhost:9090"
echo "  Grafana dashboard  : http://localhost:3000 (admin/admin)"
echo ""
echo "Prometheus queries to try:"
echo "  notifications_created_total"
echo "  notifications_delivered_total"
echo "  notifications_failed_total"
echo "  histogram_quantile(0.95, rate(notification_delivery_duration_seconds_bucket[5m]))"
echo ""
