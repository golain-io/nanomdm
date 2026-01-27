package diskv

import (
	"context"
	"fmt"

	"github.com/micromdm/nanomdm/storage"
)

// RetrieveDeviceInfo returns device information for a given enrollment ID
func (s *Diskv) RetrieveDeviceInfo(ctx context.Context, id string) (*storage.DeviceInfo, error) {
	// Diskv storage doesn't support device info retrieval
	return nil, fmt.Errorf("device info retrieval not supported by diskv storage")
}

// RetrieveAllDevices returns a list of all enrolled devices
func (s *Diskv) RetrieveAllDevices(ctx context.Context) ([]*storage.DeviceInfo, error) {
	// Diskv storage doesn't support device listing
	return nil, fmt.Errorf("device listing not supported by diskv storage")
}

// DeleteDevice removes all device-related records for a given enrollment ID
func (s *Diskv) DeleteDevice(ctx context.Context, enrollmentID string) error {
	// TODO: Implement device deletion for diskv storage
	return nil
}

// StoreBackendDeviceID stores the backend device ID mapping for an enrollment
func (s *Diskv) StoreBackendDeviceID(ctx context.Context, enrollmentID, backendDeviceID string) error {
	// TODO: Implement backend device ID storage for diskv storage
	return nil
}
