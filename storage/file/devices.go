package file

import (
	"context"
	"fmt"

	"github.com/micromdm/nanomdm/storage"
)

// RetrieveDeviceInfo returns device information for a given enrollment ID
func (s *FileStorage) RetrieveDeviceInfo(ctx context.Context, id string) (*storage.DeviceInfo, error) {
	// File storage doesn't support device info retrieval
	return nil, fmt.Errorf("device info retrieval not supported by file storage")
}

// RetrieveAllDevices returns a list of all enrolled devices
func (s *FileStorage) RetrieveAllDevices(ctx context.Context) ([]*storage.DeviceInfo, error) {
	// File storage doesn't support device listing
	return nil, fmt.Errorf("device listing not supported by file storage")
}

// DeleteDevice removes all device-related records for a given enrollment ID
func (s *FileStorage) DeleteDevice(ctx context.Context, enrollmentID string) error {
	// TODO: Implement device deletion for file storage
	return nil
}

// StoreBackendDeviceID stores the backend device ID mapping for an enrollment
func (s *FileStorage) StoreBackendDeviceID(ctx context.Context, enrollmentID, backendDeviceID string) error {
	// TODO: Implement backend device ID storage for file storage
	return nil
}
