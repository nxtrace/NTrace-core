//go:build flavor_tiny || flavor_ntr

package service

import (
	"context"
	"errors"
)

var errGlobalpingUnavailable = errors.New("globalping MCP tools are available only in the full nexttrace build")

func (s *Service) GlobalpingTrace(context.Context, GlobalpingTraceRequest) (GlobalpingMeasurementResponse, error) {
	return GlobalpingMeasurementResponse{}, errGlobalpingUnavailable
}

func (s *Service) GlobalpingLimits(context.Context, GlobalpingLimitsRequest) (GlobalpingLimitsResponse, error) {
	return GlobalpingLimitsResponse{}, errGlobalpingUnavailable
}

func (s *Service) GlobalpingGetMeasurement(context.Context, GlobalpingGetMeasurementRequest) (GlobalpingMeasurementResponse, error) {
	return GlobalpingMeasurementResponse{}, errGlobalpingUnavailable
}
