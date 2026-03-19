//go:build !flavor_tiny && !flavor_ntr

package trace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jsdelivr/globalping-cli/globalping"
)

type fakeGlobalpingClient struct {
	getMeasurement func(id string) (*globalping.Measurement, error)
	create         func(measurement *globalping.MeasurementCreate) (*globalping.MeasurementCreateResponse, error)
}

func (f fakeGlobalpingClient) CreateMeasurement(measurement *globalping.MeasurementCreate) (*globalping.MeasurementCreateResponse, error) {
	return f.create(measurement)
}

func (f fakeGlobalpingClient) GetMeasurement(id string) (*globalping.Measurement, error) {
	return f.getMeasurement(id)
}

func (f fakeGlobalpingClient) AwaitMeasurement(id string) (*globalping.Measurement, error) {
	return f.getMeasurement(id)
}

func (f fakeGlobalpingClient) GetMeasurementRaw(id string) ([]byte, error) {
	panic("not implemented")
}

func (f fakeGlobalpingClient) Authorize(func(error)) (*globalping.AuthorizeResponse, error) {
	panic("not implemented")
}

func (f fakeGlobalpingClient) TokenIntrospection(string) (*globalping.IntrospectionResponse, error) {
	panic("not implemented")
}

func (f fakeGlobalpingClient) Logout() error {
	panic("not implemented")
}

func (f fakeGlobalpingClient) RevokeToken(string) error {
	panic("not implemented")
}

func (f fakeGlobalpingClient) Limits() (*globalping.LimitsResponse, error) {
	panic("not implemented")
}

func TestAwaitGlobalpingMeasurementReturnsCanceled(t *testing.T) {
	client := fakeGlobalpingClient{
		getMeasurement: func(id string) (*globalping.Measurement, error) {
			return &globalping.Measurement{Status: globalping.StatusInProgress}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := awaitGlobalpingMeasurement(ctx, client, "m-1")
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("awaitGlobalpingMeasurement error = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("awaitGlobalpingMeasurement did not return promptly after cancel")
	}
}

func TestCreateGlobalpingMeasurementHonorsCanceledContext(t *testing.T) {
	client := fakeGlobalpingClient{
		create: func(measurement *globalping.MeasurementCreate) (*globalping.MeasurementCreateResponse, error) {
			return &globalping.MeasurementCreateResponse{ID: "m-1"}, nil
		},
		getMeasurement: func(id string) (*globalping.Measurement, error) {
			return &globalping.Measurement{Status: globalping.StatusInProgress}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := createGlobalpingMeasurement(ctx, client, &globalping.MeasurementCreate{})
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("createGlobalpingMeasurement error = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("createGlobalpingMeasurement did not return promptly after cancel")
	}
}
