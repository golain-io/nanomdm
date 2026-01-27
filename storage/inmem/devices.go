package inmem

import (
	"context"
	"fmt"

	"github.com/micromdm/nanomdm/storage"
)

// RetrieveDeviceInfo returns device information for a given enrollment ID
func (s *InMem) RetrieveDeviceInfo(ctx context.Context, id string) (*storage.DeviceInfo, error) {
	// In-memory storage doesn't support device info retrieval
	return nil, fmt.Errorf("device info retrieval not supported by in-memory storage")
}

// RetrieveAllDevices returns a list of all enrolled devices
func (s *InMem) RetrieveAllDevices(ctx context.Context) ([]*storage.DeviceInfo, error) {
	// In-memory storage doesn't support device listing
	return nil, fmt.Errorf("device listing not supported by in-memory storage")
}

// DeleteDevice removes all device-related records for a given enrollment ID
func (s *InMem) DeleteDevice(ctx context.Context, enrollmentID string) error {
	// TODO: Implement device deletion for in-memory storage
	return nil
}

// StoreBackendDeviceID stores the backend device ID mapping for an enrollment
func (s *InMem) StoreBackendDeviceID(ctx context.Context, enrollmentID, backendDeviceID string) error {
	// TODO: Implement backend device ID storage for in-memory storage
	return nil
}
