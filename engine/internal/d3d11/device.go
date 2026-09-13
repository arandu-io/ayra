package d3d11

import (
	"errors"
	"fmt"
)

const (
	DRIVER_TYPE_HARDWARE = 1
	DRIVER_TYPE_WARP     = 5
)

func withDriverFallback[T any](create func(driverType uint32) (T, error)) (T, error) {
	device, hardwareErr := create(DRIVER_TYPE_HARDWARE)
	if hardwareErr == nil {
		return device, nil
	}
	device, warpErr := create(DRIVER_TYPE_WARP)
	if warpErr == nil {
		return device, nil
	}
	var zero T
	return zero, errors.Join(
		fmt.Errorf("hardware D3D11 device: %w", hardwareErr),
		fmt.Errorf("WARP D3D11 device: %w", warpErr),
	)
}
