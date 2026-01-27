// Package storage defines interfaces, types, data, and helpers related
// to storage and retrieval for MDM enrollments and commands.
package storage

import (
	"context"
	"errors"
)

// ErrDeviceChannelOnly is returned when storage operations are only possible on the device MDM channel.
var ErrDeviceChannelOnly = errors.New("operation supported on device channel only")

// AllStorage represents all required storage by NanoMDM.
type AllStorage interface {
	ServiceStore
	PushStore
	PushCertStore
	CommandEnqueuer
	CertAuthStore
	CertAuthRetriever
	StoreMigrator
	TokenUpdateTallyStore
	DeviceInfoStore
	// StoreBackendDeviceID stores the backend device ID mapping for an enrollment
	StoreBackendDeviceID(ctx context.Context, enrollmentID, backendDeviceID string) error
}

// ServiceStore stores & retrieves both command and check-in data.
type ServiceStore interface {
	CheckinStore
	CommandAndReportResultsStore
	BootstrapTokenStore
}

// DeviceInfo represents device information for API responses
type DeviceInfo struct {
	ID                string                 `json:"id"`
	SerialNumber      string                 `json:"serial_number,omitempty"`
	Type              string                 `json:"type,omitempty"`
	Enabled           bool                   `json:"enabled"`
	LastSeen          string                 `json:"last_seen,omitempty"`
	AuthenticatedAt   string                 `json:"authenticated_at,omitempty"`
	TokenUpdateTally  int                    `json:"token_update_tally,omitempty"`
	Topic             string                 `json:"topic,omitempty"`
	BackendDeviceID   string                 `json:"backend_device_id,omitempty"`
	HardwareInfo      map[string]interface{} `json:"hardware_info,omitempty"`
	NetworkInfo       map[string]interface{} `json:"network_info,omitempty"`
	StorageInfo       map[string]interface{} `json:"storage_info,omitempty"`
	PowerInfo         map[string]interface{} `json:"power_info,omitempty"`
	ManagementInfo    map[string]interface{} `json:"management_info,omitempty"`
	LastDeviceQuery   string                 `json:"last_device_query,omitempty"`
}

// DeviceInfoStore defines methods for retrieving device information
type DeviceInfoStore interface {
	// RetrieveDeviceInfo returns device information for a given enrollment ID
	RetrieveDeviceInfo(ctx context.Context, id string) (*DeviceInfo, error)
	// RetrieveAllDevices returns a list of all enrolled devices
	RetrieveAllDevices(ctx context.Context) ([]*DeviceInfo, error)
	// DeleteDevice removes all device-related records for a given enrollment ID
	DeleteDevice(ctx context.Context, enrollmentID string) error
}
