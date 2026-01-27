package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/micromdm/nanomdm/storage"
)

// RetrieveDeviceInfo returns device information for a given enrollment ID
func (s *MySQLStorage) RetrieveDeviceInfo(ctx context.Context, id string) (*storage.DeviceInfo, error) {
	query := `
		SELECT 
			e.id,
			d.serial_number,
			e.type,
			e.enabled,
			e.last_seen_at,
			d.authenticate_at,
			e.token_update_tally,
			e.topic
		FROM enrollments e
		LEFT JOIN devices d ON e.device_id = d.id
		WHERE e.id = ?
		LIMIT 1
	`
	
	var device storage.DeviceInfo
	var lastSeen, authenticatedAt sql.NullTime
	
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&device.ID,
		&device.SerialNumber,
		&device.Type,
		&device.Enabled,
		&lastSeen,
		&authenticatedAt,
		&device.TokenUpdateTally,
		&device.Topic,
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
	
	return &device, nil
}

// RetrieveAllDevices returns a list of all enrolled devices
func (s *MySQLStorage) RetrieveAllDevices(ctx context.Context) ([]*storage.DeviceInfo, error) {
	query := `
		SELECT 
			e.id,
			d.serial_number,
			e.type,
			e.enabled,
			e.last_seen_at,
			d.authenticate_at,
			e.token_update_tally,
			e.topic
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
		
		err := rows.Scan(
			&device.ID,
			&device.SerialNumber,
			&device.Type,
			&device.Enabled,
			&lastSeen,
			&authenticatedAt,
			&device.TokenUpdateTally,
			&device.Topic,
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
		
		devices = append(devices, &device)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating device rows: %w", err)
	}
	
	return devices, nil
}

// DeleteDevice removes all device-related records for a given enrollment ID
func (s *MySQLStorage) DeleteDevice(ctx context.Context, enrollmentID string) error {
	// TODO: Implement device deletion for MySQL
	return nil
}

// StoreBackendDeviceID stores the backend device ID mapping for an enrollment
func (s *MySQLStorage) StoreBackendDeviceID(ctx context.Context, enrollmentID, backendDeviceID string) error {
	// TODO: Implement backend device ID storage for MySQL
	return nil
}
