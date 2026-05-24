#!/bin/bash
set -e

echo "Waiting for Kafka to be ready..."
sleep 10

KAFKA_BROKER="kafka:29092"
TOPICS=(
  "notifications-sms-high" "notifications-sms-normal" "notifications-sms-low"
  "notifications-email-high" "notifications-email-normal" "notifications-email-low"
  "notifications-push-high" "notifications-push-normal" "notifications-push-low"
  "notifications-retry" "notifications-dlq"
)

for TOPIC in "${TOPICS[@]}"; do
  echo "Creating topic: $TOPIC"
  /opt/kafka/bin/kafka-topics.sh \
    --create \
    --if-not-exists \
    --bootstrap-server "$KAFKA_BROKER" \
    --topic "$TOPIC" \
    --partitions 3 \
    --replication-factor 1
done

echo "All topics created successfully."
