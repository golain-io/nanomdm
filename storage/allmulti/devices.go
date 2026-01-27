package allmulti

import (
	"context"
	"fmt"

	"github.com/micromdm/nanomdm/storage"
)

// RetrieveDeviceInfo returns device information for a given enrollment ID
func (s *MultiAllStorage) RetrieveDeviceInfo(ctx context.Context, id string) (*storage.DeviceInfo, error) {
	// Try each storage backend until one succeeds
	for i, store := range s.stores {
		info, err := store.RetrieveDeviceInfo(ctx, id)
		if err == nil {
			return info, nil
		}
		s.logger.Debug("msg", "storage backend failed", "backend", i, "err", err)
	}
	return nil, fmt.Errorf("all storage backends failed")
}

// RetrieveAllDevices returns a list of all enrolled devices
func (s *MultiAllStorage) RetrieveAllDevices(ctx context.Context) ([]*storage.DeviceInfo, error) {
	// Try each storage backend until one succeeds
	for i, store := range s.stores {
		devices, err := store.RetrieveAllDevices(ctx)
		if err == nil {
			return devices, nil
		}
		s.logger.Debug("msg", "storage backend failed", "backend", i, "err", err)
	}
	return nil, fmt.Errorf("all storage backends failed")
}

// DeleteDevice removes all device-related records for a given enrollment ID
func (s *MultiAllStorage) DeleteDevice(ctx context.Context, enrollmentID string) error {
	// Try each storage backend until one succeeds
	for i, store := range s.stores {
		err := store.DeleteDevice(ctx, enrollmentID)
		if err == nil {
			return nil
		}
		s.logger.Debug("msg", "storage backend failed", "backend", i, "err", err)
	}
	return fmt.Errorf("all storage backends failed")
}

// StoreBackendDeviceID stores the backend device ID mapping for an enrollment
func (s *MultiAllStorage) StoreBackendDeviceID(ctx context.Context, enrollmentID, backendDeviceID string) error {
	// Try each storage backend until one succeeds
	for i, store := range s.stores {
		err := store.StoreBackendDeviceID(ctx, enrollmentID, backendDeviceID)
		if err == nil {
			return nil
		}
		s.logger.Debug("msg", "storage backend failed", "backend", i, "err", err)
	}
	return fmt.Errorf("all storage backends failed")
}
