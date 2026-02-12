#!/usr/bin/env python3
"""
Simple RabbitMQ consumer to verify NanoMDM AMQP messages.
Shows if device_id appears in url_params of TokenUpdate events.

To run this script:
  docker build -f Dockerfile.test-rabbitmq -t test-rabbitmq .
  docker run --rm --network queue -it test-rabbitmq

Or run directly with docker run:
  docker run --rm --network queue -v $(pwd)/certs:/app/certs -v $(pwd)/test-rabbitmq.py:/app/test-rabbitmq.py python:3.11-slim sh -c "pip install pika && python3 /app/test-rabbitmq.py"
"""
import pika
import json
import ssl
import sys
import os

# RabbitMQ connection details
# RabbitMQ container name: queue
# Web UI: https://staging-rabbitmq.golain.io/#/exchanges
# AMQPS: queue:5671 (from within Docker network)
RABBITMQ_HOST = 'queue'  # Container name on Docker network
RABBITMQ_PORT = 5671
RABBITMQ_USER = 'rabbitmq'
RABBITMQ_PASS = 'ef19e2ji'

# Certificate paths (adjust if needed)
CA_CERT = 'certs/rabbitmq-ca.pem'
CLIENT_CERT = 'certs/rabbitmq-client.pem'
CLIENT_KEY = 'certs/rabbitmq-client.key'

def main():
    # Check cert files exist
    for cert_file in [CA_CERT, CLIENT_CERT, CLIENT_KEY]:
        if not os.path.exists(cert_file):
            print(f"❌ Certificate file not found: {cert_file}")
            print("   Make sure you're running from the nanomdm directory")
            sys.exit(1)
    
    # TLS setup
    context = ssl.create_default_context(cafile=CA_CERT)
    context.load_cert_chain(CLIENT_CERT, CLIENT_KEY)
    
    # Connect
    credentials = pika.PlainCredentials(RABBITMQ_USER, RABBITMQ_PASS)
    parameters = pika.ConnectionParameters(
        host=RABBITMQ_HOST,
        port=RABBITMQ_PORT,
        virtual_host='/',
        credentials=credentials,
        ssl_options=pika.SSLOptions(context)
    )
    
    print("🔌 Connecting to RabbitMQ...")
    try:
        connection = pika.BlockingConnection(parameters)
        channel = connection.channel()
        print("✅ Connected successfully!\n")
    except Exception as e:
        print(f"❌ Connection failed: {e}")
        sys.exit(1)
    
    # Declare exchange (if not exists)
    channel.exchange_declare(exchange='mdm_exchange', exchange_type='topic', durable=True)
    
    # Create a temporary queue
    queue_name = 'test_mdm_events'
    result = channel.queue_declare(queue=queue_name, durable=True)
    
    # Bind queue to exchange with routing key
    channel.queue_bind(exchange='mdm_exchange', queue=queue_name, routing_key='mdm.event')
    
    print(f"📬 Listening on queue: {queue_name}")
    print(f"📡 Exchange: mdm_exchange")
    print(f"🔑 Routing key: mdm.event")
    print("\n" + "="*80)
    print("Waiting for MDM events...")
    print("(Trigger a device check-in to see messages)")
    print("Press CTRL+C to exit")
    print("="*80 + "\n")
    
    def callback(ch, method, properties, body):
        try:
            # Parse JSON message
            event = json.loads(body.decode('utf-8'))
            
            # Extract key info
            topic = event.get('Topic', event.get('topic', 'unknown'))
            checkin = event.get('CheckinEvent', event.get('checkin_event', {}))
            udid = checkin.get('UDID', checkin.get('udid', 'N/A'))
            url_params = checkin.get('Params', checkin.get('url_params', {}))
            device_id = url_params.get('device_id') if isinstance(url_params, dict) else None
            
            # Print summary
            print("📨 " + "="*78)
            print(f"   Event Type: {topic}")
            print(f"   UDID:       {udid}")
            
            if device_id:
                print(f"   ✅ device_id: {device_id}")
                print("   Status:     SUCCESS - device_id found in url_params!")
            else:
                print(f"   ❌ device_id: MISSING")
                print("   Status:     FAIL - device_id not in url_params")
                if url_params:
                    print(f"   url_params: {url_params}")
                else:
                    print("   url_params: (empty or missing)")
            
            # Show full event structure (truncated if very long)
            print("\n   Full event structure:")
            event_str = json.dumps(event, indent=2)
            if len(event_str) > 1000:
                print(event_str[:1000] + "\n   ... (truncated)")
            else:
                print(event_str)
            
            print("="*80 + "\n")
            
        except json.JSONDecodeError as e:
            print(f"❌ Error parsing JSON: {e}")
            print(f"Raw body (first 200 chars): {body[:200]}")
            print()
        except Exception as e:
            print(f"❌ Error: {e}")
            print(f"Raw body (first 200 chars): {body[:200]}")
            print()
    
    channel.basic_consume(queue=queue_name, on_message_callback=callback, auto_ack=True)
    
    try:
        channel.start_consuming()
    except KeyboardInterrupt:
        print("\n\n👋 Stopping consumer...")
        channel.stop_consuming()
        connection.close()
        print("✅ Disconnected")

if __name__ == '__main__':
    main()
