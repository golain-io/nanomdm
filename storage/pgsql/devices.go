package pgsql

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"regexp"
	"strconv"

	"github.com/micromdm/nanomdm/storage"
)

// RetrieveDeviceInfo returns device information for a given enrollment ID
func (s *PgSQLStorage) RetrieveDeviceInfo(ctx context.Context, id string) (*storage.DeviceInfo, error) {
	query := `
		SELECT 
			e.id,
			d.serial_number,
			e.type,
			e.enabled,
			e.last_seen_at,
			d.authenticate_at,
			e.token_update_tally,
			e.topic,
			d.backend_device_id
		FROM enrollments e
		LEFT JOIN devices d ON e.device_id = d.id
		WHERE e.id = $1
		LIMIT 1
	`
	
	var device storage.DeviceInfo
	var lastSeen, authenticatedAt sql.NullTime
	var backendDeviceID sql.NullString
	
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&device.ID,
		&device.SerialNumber,
		&device.Type,
		&device.Enabled,
		&lastSeen,
		&authenticatedAt,
		&device.TokenUpdateTally,
		&device.Topic,
		&backendDeviceID,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Device not found
		}
		return nil, fmt.Errorf("retrieving device info: %w", err)
	}
	
	// Convert timestamps to strings
	if lastSeen.Valid {
		device.LastSeen = lastSeen.Time.Format(time.RFC3339)
	}
	if authenticatedAt.Valid {
		device.AuthenticatedAt = authenticatedAt.Time.Format(time.RFC3339)
	}
	
	// Set backend device ID if available
	if backendDeviceID.Valid {
		device.BackendDeviceID = backendDeviceID.String
	}
	
	// Get latest DeviceInformation command result with most fields
	// Prioritize auto-generated commands which have more comprehensive data
	deviceQuery := `
		SELECT cr.result, cr.created_at
		FROM command_results cr
		JOIN commands c ON cr.command_uuid = c.command_uuid
		WHERE cr.id = $1
			AND c.request_type = 'DeviceInformation'
			AND cr.status = 'Acknowledged'
		ORDER BY 
			CASE WHEN c.command_uuid LIKE 'auto-device-info-%' THEN 0 ELSE 1 END,
			cr.created_at DESC
		LIMIT 1
	`
	
	var xmlResult sql.NullString
	var queryTime sql.NullTime
	
	err = s.db.QueryRowContext(ctx, deviceQuery, id).Scan(&xmlResult, &queryTime)
	if err == nil && xmlResult.Valid {
		device.LastDeviceQuery = queryTime.Time.Format(time.RFC3339)
		
		// Parse the XML and extract device information
		deviceInfo := parseDeviceInfoXML(xmlResult.String)
		if deviceInfo != nil {
			device.HardwareInfo = deviceInfo["hardware"]
			device.NetworkInfo = deviceInfo["network"]
			device.StorageInfo = deviceInfo["storage"]
			device.PowerInfo = deviceInfo["power"]
			device.ManagementInfo = deviceInfo["management"]
		}
	}
	
	return &device, nil
}

// RetrieveAllDevices returns a list of all enrolled devices
func (s *PgSQLStorage) RetrieveAllDevices(ctx context.Context) ([]*storage.DeviceInfo, error) {
	query := `
		SELECT 
			e.id,
			d.serial_number,
			e.type,
			e.enabled,
			e.last_seen_at,
			d.authenticate_at,
			e.token_update_tally,
			e.topic,
			d.backend_device_id
		FROM enrollments e
		LEFT JOIN devices d ON e.device_id = d.id
		ORDER BY e.last_seen_at DESC
	`
	
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying all devices: %w", err)
	}
	defer rows.Close()
	
	var devices []*storage.DeviceInfo
	
	for rows.Next() {
		var device storage.DeviceInfo
		var lastSeen, authenticatedAt sql.NullTime
		var backendDeviceID sql.NullString
		
		err := rows.Scan(
			&device.ID,
			&device.SerialNumber,
			&device.Type,
			&device.Enabled,
			&lastSeen,
			&authenticatedAt,
			&device.TokenUpdateTally,
			&device.Topic,
			&backendDeviceID,
		)
		
		if err != nil {
			return nil, fmt.Errorf("scanning device row: %w", err)
		}
		
		// Convert timestamps to strings
		if lastSeen.Valid {
			device.LastSeen = lastSeen.Time.Format(time.RFC3339)
		}
		if authenticatedAt.Valid {
			device.AuthenticatedAt = authenticatedAt.Time.Format(time.RFC3339)
		}
		
		// Set backend device ID if available
		if backendDeviceID.Valid {
			device.BackendDeviceID = backendDeviceID.String
		}
		
		devices = append(devices, &device)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating device rows: %w", err)
	}
	
	return devices, nil
}

// parseDeviceInfoXML parses DeviceInformation XML and returns structured data
func parseDeviceInfoXML(xmlString string) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	
	// Initialize result maps
	hardware := make(map[string]interface{})
	network := make(map[string]interface{})
	storage := make(map[string]interface{})
	power := make(map[string]interface{})
	management := make(map[string]interface{})
	
	// Helper function to extract string values using regex
	extractString := func(key string) string {
		pattern := fmt.Sprintf(`<key>%s</key>\s*<string>([^<]*)</string>`, regexp.QuoteMeta(key))
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(xmlString)
		if len(matches) > 1 {
			return matches[1]
		}
		return ""
	}
	
	// Helper function to extract real/number values
	extractReal := func(key string) float64 {
		pattern := fmt.Sprintf(`<key>%s</key>\s*<real>([^<]*)</real>`, regexp.QuoteMeta(key))
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(xmlString)
		if len(matches) > 1 {
			if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
				return val
			}
		}
		return 0
	}
	
	// Helper function to extract boolean values
	extractBool := func(key string) bool {
		pattern := fmt.Sprintf(`<key>%s</key>\s*<(true|false)/>`, regexp.QuoteMeta(key))
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(xmlString)
		if len(matches) > 1 {
			return matches[1] == "true"
		}
		return false
	}
	
	// Extract hardware information
	hardware["model"] = extractString("Model")
	hardware["product_name"] = extractString("ProductName")
	hardware["device_name"] = extractString("DeviceName")
	hardware["serial_number"] = extractString("SerialNumber")
	hardware["udid"] = extractString("UDID")
	hardware["os_version"] = extractString("OSVersion")
	hardware["build_version"] = extractString("BuildVersion")
	hardware["imei"] = extractString("IMEI")
	hardware["meid"] = extractString("MEID")
	hardware["iccid"] = extractString("ICCID")
	hardware["modem_firmware_version"] = extractString("ModemFirmwareVersion")
	
	// Extract network information
	network["wifi_mac"] = extractString("WiFiMAC")
	network["bluetooth_mac"] = extractString("BluetoothMAC")
	network["phone_number"] = extractString("PhoneNumber")
	network["is_roaming"] = extractBool("IsRoaming")
	network["data_roaming_enabled"] = extractBool("DataRoamingEnabled")
	network["personal_hotspot_enabled"] = extractBool("PersonalHotspotEnabled")
	
	// Extract storage information
	storage["available_capacity_gb"] = extractReal("AvailableDeviceCapacity")
	// storage["total_capacity_gb"] = extractReal("DeviceCapacity")
	// storage["total_disk_capacity"] = extractReal("TotalDiskCapacity")
	
	// Extract additional hardware fields
	hardware["host_name"] = extractString("HostName")
	hardware["local_host_name"] = extractString("LocalHostName")
	hardware["current_console_managed_user"] = extractString("CurrentConsoleManagedUser")
	
	// Extract power information
	power["battery_level"] = extractReal("BatteryLevel")
	
	// Extract management information
	management["is_supervised"] = extractBool("IsSupervised")
	management["is_activation_lock_enabled"] = extractBool("IsActivationLockEnabled")
	management["is_device_locator_enabled"] = extractBool("IsDeviceLocatorServiceEnabled")
	management["is_do_not_disturb"] = extractBool("IsDoNotDisturbInEffect")
	management["is_cloud_backup_enabled"] = extractBool("IsCloudBackupEnabled")
	management["diagnostic_submission_enabled"] = extractBool("DiagnosticSubmissionEnabled")
	management["app_analytics_enabled"] = extractBool("AppAnalyticsEnabled")
	
	result["hardware"] = hardware
	result["network"] = network
	result["storage"] = storage
	result["power"] = power
	result["management"] = management
	
	return result
}

// DeleteDevice removes all device-related records for a given enrollment ID
func (s *PgSQLStorage) DeleteDevice(ctx context.Context, enrollmentID string) error {
	// Start a transaction to ensure atomicity
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Get device ID for this enrollment
	var deviceID string
	err = tx.QueryRowContext(ctx, 
		"SELECT device_id FROM enrollments WHERE id = $1", 
		enrollmentID,
	).Scan(&deviceID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Device already deleted or doesn't exist
			return nil
		}
		return fmt.Errorf("getting device_id: %w", err)
	}

	// Delete command results for this device
	_, err = tx.ExecContext(ctx, 
		"DELETE FROM command_results WHERE id = $1", 
		enrollmentID,
	)
	if err != nil {
		return fmt.Errorf("deleting command results: %w", err)
	}

	// Delete enrollment queue entries for this device
	_, err = tx.ExecContext(ctx, 
		"DELETE FROM enrollment_queue WHERE id = $1", 
		enrollmentID,
	)
	if err != nil {
		return fmt.Errorf("deleting enrollment queue: %w", err)
	}

	// Delete enrollments for this device
	_, err = tx.ExecContext(ctx, 
		"DELETE FROM enrollments WHERE device_id = $1", 
		deviceID,
	)
	if err != nil {
		return fmt.Errorf("deleting enrollments: %w", err)
	}

	// Delete users for this device
	_, err = tx.ExecContext(ctx, 
		"DELETE FROM users WHERE device_id = $1", 
		deviceID,
	)
	if err != nil {
		return fmt.Errorf("deleting users: %w", err)
	}

	// Delete device record
	_, err = tx.ExecContext(ctx, 
		"DELETE FROM devices WHERE id = $1", 
		deviceID,
	)
	if err != nil {
		return fmt.Errorf("deleting device: %w", err)
	}

	// Delete cert auth associations for this device
	_, err = tx.ExecContext(ctx, 
		"DELETE FROM cert_auth_associations WHERE enrollment_id = $1", 
		enrollmentID,
	)
	if err != nil {
		return fmt.Errorf("deleting cert auth associations: %w", err)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// StoreBackendDeviceID stores the backend device ID mapping for an enrollment
func (s *PgSQLStorage) StoreBackendDeviceID(ctx context.Context, enrollmentID, backendDeviceID string) error {
	// Get the device ID for this enrollment
	var deviceID string
	err := s.db.QueryRowContext(ctx, 
		"SELECT device_id FROM enrollments WHERE id = $1", 
		enrollmentID,
	).Scan(&deviceID)
	if err != nil {
		return fmt.Errorf("getting device_id: %w", err)
	}

	// Update the device record with the backend device ID
	_, err = s.db.ExecContext(ctx, 
		"UPDATE devices SET backend_device_id = $1 WHERE id = $2", 
		backendDeviceID, deviceID,
	)
	if err != nil {
		return fmt.Errorf("updating backend_device_id: %w", err)
	}

	return nil
}
