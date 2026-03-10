#!/usr/bin/env python3
"""
Enqueue DeviceInformation command for all enrolled devices.
This is useful for devices enrolled before -auto-device-info was enabled.
"""
import requests
import uuid
import time
import sys

# Configuration
NANOMDM_URL = "https://iosmdm.msbfc.in"
API_USER = "nanomdm"
API_KEY = "YOUR_API_KEY_HERE"  # Update this!

def get_all_devices():
    """Get list of all enrolled devices."""
    url = f"{NANOMDM_URL}/v1/devices"
    resp = requests.get(url, auth=(API_USER, API_KEY))
    resp.raise_for_status()
    return resp.json()

def enqueue_device_information(enrollment_id):
    """Enqueue DeviceInformation command for a device."""
    cmd_uuid = str(uuid.uuid4())
    
    plist = f'''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Command</key>
  <dict>
    <key>RequestType</key>
    <string>DeviceInformation</string>
  </dict>
  <key>CommandUUID</key>
  <string>{cmd_uuid}</string>
</dict>
</plist>'''
    
    url = f"{NANOMDM_URL}/v1/enqueue/{enrollment_id}"
    resp = requests.put(
        url,
        auth=(API_USER, API_KEY),
        headers={"Content-Type": "text/plain"},
        data=plist
    )
    resp.raise_for_status()
    return resp.json()

def main():
    if API_KEY == "YOUR_API_KEY_HERE":
        print("❌ Please update API_KEY in the script!")
        sys.exit(1)
    
    print("📋 Fetching all enrolled devices...")
    devices = get_all_devices()
    
    if not devices:
        print("✅ No devices found")
        return
    
    print(f"📱 Found {len(devices)} device(s)")
    
    for device in devices:
        enrollment_id = device.get("id")
        serial = device.get("serial_number", "N/A")
        has_metadata = device.get("last_device_query") is not None
        
        if has_metadata:
            print(f"⏭️  Skipping {enrollment_id} ({serial}) - already has metadata")
            continue
        
        print(f"📤 Enqueueing DeviceInformation for {enrollment_id} ({serial})...")
        try:
            result = enqueue_device_information(enrollment_id)
            if result.get("status", {}).get(enrollment_id, {}).get("push_result"):
                print(f"   ✅ Queued + pushed")
            else:
                print(f"   ⚠️  Queued but push may have failed")
        except Exception as e:
            print(f"   ❌ Error: {e}")
        
        time.sleep(0.5)  # Rate limit protection
    
    print("\n✅ Done! Devices will check in and metadata will appear soon.")
    print("   Check with: curl -sS -u 'nanomdm:API_KEY' 'https://iosmdm.msbfc.in/v1/devices/<ID>'")

if __name__ == "__main__":
    main()
