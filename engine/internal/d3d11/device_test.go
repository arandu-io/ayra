package d3d11

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestWithDriverFallbackStopsAfterHardwareSuccess(t *testing.T) {
	var calls []uint32
	got, err := withDriverFallback(func(driverType uint32) (string, error) {
		calls = append(calls, driverType)
		return "hardware", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "hardware" {
		t.Fatalf("got device %q, want hardware", got)
	}
	if want := []uint32{DRIVER_TYPE_HARDWARE}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("got driver order %v, want %v", calls, want)
	}
}

func TestWithDriverFallbackUsesWARPAfterHardwareFailure(t *testing.T) {
	hardwareErr := errors.New("hardware unavailable")
	var calls []uint32
	got, err := withDriverFallback(func(driverType uint32) (string, error) {
		calls = append(calls, driverType)
		if driverType == DRIVER_TYPE_HARDWARE {
			return "", hardwareErr
		}
		return "warp", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "warp" {
		t.Fatalf("got device %q, want warp", got)
	}
	if want := []uint32{DRIVER_TYPE_HARDWARE, DRIVER_TYPE_WARP}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("got driver order %v, want %v", calls, want)
	}
}

func TestWithDriverFallbackPreservesBothFailures(t *testing.T) {
	hardwareErr := errors.New("hardware unavailable")
	warpErr := errors.New("warp unavailable")
	var calls []uint32
	got, err := withDriverFallback(func(driverType uint32) (string, error) {
		calls = append(calls, driverType)
		if driverType == DRIVER_TYPE_HARDWARE {
			return "", hardwareErr
		}
		return "", warpErr
	})
	if got != "" {
		t.Fatalf("got device %q after both failures", got)
	}
	if !errors.Is(err, hardwareErr) {
		t.Fatalf("error %q does not preserve the hardware failure", err)
	}
	if !errors.Is(err, warpErr) {
		t.Fatalf("error %q does not preserve the WARP failure", err)
	}
	if !strings.Contains(err.Error(), "hardware D3D11 device") || !strings.Contains(err.Error(), "WARP D3D11 device") {
		t.Fatalf("error %q does not identify both driver attempts", err)
	}
	if want := []uint32{DRIVER_TYPE_HARDWARE, DRIVER_TYPE_WARP}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("got driver order %v, want %v", calls, want)
	}
}
