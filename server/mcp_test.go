package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nxtrace/NTrace-core/internal/service"
)

type fakeMCPService struct{}

func (fakeMCPService) Capabilities(context.Context, service.CapabilitiesRequest) (service.CapabilitiesResponse, error) {
	return service.CapabilitiesResponse{
		Tools: []service.ToolCapability{{
			Name:        "nexttrace_globalping_trace",
			Description: "globalping",
			Parameters:  service.ParameterBoundaries{Supported: []string{"target", "locations"}},
		}},
	}, nil
}

func (fakeMCPService) Traceroute(context.Context, service.TraceRequest) (service.TraceResponse, error) {
	return service.TraceResponse{Target: "example.com", ResolvedIP: "93.184.216.34", Protocol: "icmp"}, nil
}

func (fakeMCPService) MTRReport(context.Context, service.MTRReportRequest) (service.MTRReportResponse, error) {
	return service.MTRReportResponse{Target: "example.com", ResolvedIP: "93.184.216.34", Protocol: "icmp"}, nil
}

func (fakeMCPService) MTRRaw(context.Context, service.MTRRawRequest) (service.MTRRawResponse, error) {
	return service.MTRRawResponse{Target: "example.com", ResolvedIP: "93.184.216.34", Protocol: "icmp"}, nil
}

func (fakeMCPService) MTUTrace(context.Context, service.MTUTraceRequest) (service.MTUTraceResponse, error) {
	return service.MTUTraceResponse{Target: "example.com", ResolvedIP: "93.184.216.34", Protocol: "udp"}, nil
}

func (fakeMCPService) SpeedTest(context.Context, service.SpeedTestRequest) (service.SpeedTestResponse, error) {
	return service.SpeedTestResponse{}, nil
}

func (fakeMCPService) AnnotateIPs(context.Context, service.AnnotateIPsRequest) (service.AnnotateIPsResponse, error) {
	return service.AnnotateIPsResponse{Text: "8.8.8.8 [ok]"}, nil
}

func (fakeMCPService) GeoLookup(context.Context, service.GeoLookupRequest) (service.GeoLookupResponse, error) {
	return service.GeoLookupResponse{Query: "8.8.8.8"}, nil
}

func (fakeMCPService) GlobalpingTrace(context.Context, service.GlobalpingTraceRequest) (service.GlobalpingMeasurementResponse, error) {
	return service.GlobalpingMeasurementResponse{MeasurementID: "m-1", Status: "finished", ProbesCount: 2}, nil
}

func (fakeMCPService) GlobalpingLimits(context.Context, service.GlobalpingLimitsRequest) (service.GlobalpingLimitsResponse, error) {
	return service.GlobalpingLimitsResponse{Measurements: service.GlobalpingMeasurementLimits{Create: service.GlobalpingCreateLimit{Limit: 10, Remaining: 9}}}, nil
}

func (fakeMCPService) GlobalpingGetMeasurement(context.Context, service.GlobalpingGetMeasurementRequest) (service.GlobalpingMeasurementResponse, error) {
	return service.GlobalpingMeasurementResponse{MeasurementID: "m-1", Status: "finished"}, nil
}

func TestMCPHandlerRegistersAndCallsTools(t *testing.T) {
	ts := httptest.NewServer(newMCPHTTPHandlerWithService(fakeMCPService{}))
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	seen := map[string]bool{}
	for _, tool := range tools.Tools {
		seen[tool.Name] = true
	}
	for _, name := range []string{
		"nexttrace_capabilities",
		"nexttrace_traceroute",
		"nexttrace_globalping_trace",
		"nexttrace_globalping_limits",
		"nexttrace_globalping_get_measurement",
	} {
		if !seen[name] {
			t.Fatalf("tool %s not registered; got %#v", name, seen)
		}
	}

	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"nexttrace_capabilities", map[string]any{}},
		{"nexttrace_traceroute", map[string]any{"target": "example.com"}},
		{"nexttrace_globalping_trace", map[string]any{"target": "example.com", "locations": []string{"Japan"}}},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		if err != nil {
			t.Fatalf("CallTool(%s) returned error: %v", call.name, err)
		}
		if result.IsError {
			t.Fatalf("CallTool(%s) returned tool error: %#v", call.name, result.Content)
		}
		if len(result.Content) == 0 {
			t.Fatalf("CallTool(%s) returned no content", call.name)
		}
		if result.StructuredContent == nil {
			t.Fatalf("CallTool(%s) returned nil structured content", call.name)
		}
	}
}
