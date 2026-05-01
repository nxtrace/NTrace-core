package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nxtrace/NTrace-core/internal/service"
)

type recordingMCPService struct {
	calls    map[string]int
	inputs   map[string]any
	failTool string
}

func newRecordingMCPService() *recordingMCPService {
	return &recordingMCPService{
		calls:  make(map[string]int),
		inputs: make(map[string]any),
	}
}

func (s *recordingMCPService) record(name string, input any) error {
	s.calls[name]++
	s.inputs[name] = input
	if s.failTool == name {
		return errors.New("forced " + name)
	}
	return nil
}

func (s *recordingMCPService) Capabilities(_ context.Context, input service.CapabilitiesRequest) (service.CapabilitiesResponse, error) {
	if err := s.record("nexttrace_capabilities", input); err != nil {
		return service.CapabilitiesResponse{}, err
	}
	return service.CapabilitiesResponse{
		Tools: []service.ToolCapability{{
			Name:        "nexttrace_globalping_trace",
			Description: "globalping",
			Parameters:  service.ParameterBoundaries{Supported: []string{"target", "locations"}},
		}},
	}, nil
}

func (s *recordingMCPService) Traceroute(_ context.Context, input service.TraceRequest) (service.TraceResponse, error) {
	if err := s.record("nexttrace_traceroute", input); err != nil {
		return service.TraceResponse{}, err
	}
	return service.TraceResponse{Target: "example.com", ResolvedIP: "93.184.216.34", Protocol: "icmp"}, nil
}

func (s *recordingMCPService) MTRReport(_ context.Context, input service.MTRReportRequest) (service.MTRReportResponse, error) {
	if err := s.record("nexttrace_mtr_report", input); err != nil {
		return service.MTRReportResponse{}, err
	}
	return service.MTRReportResponse{Target: "example.com", ResolvedIP: "93.184.216.34", Protocol: "icmp"}, nil
}

func (s *recordingMCPService) MTRRaw(_ context.Context, input service.MTRRawRequest) (service.MTRRawResponse, error) {
	if err := s.record("nexttrace_mtr_raw", input); err != nil {
		return service.MTRRawResponse{}, err
	}
	return service.MTRRawResponse{Target: "example.com", ResolvedIP: "93.184.216.34", Protocol: "icmp"}, nil
}

func (s *recordingMCPService) MTUTrace(_ context.Context, input service.MTUTraceRequest) (service.MTUTraceResponse, error) {
	if err := s.record("nexttrace_mtu_trace", input); err != nil {
		return service.MTUTraceResponse{}, err
	}
	return service.MTUTraceResponse{Target: "example.com", ResolvedIP: "93.184.216.34", Protocol: "udp"}, nil
}

func (s *recordingMCPService) SpeedTest(_ context.Context, input service.SpeedTestRequest) (service.SpeedTestResponse, error) {
	if err := s.record("nexttrace_speed_test", input); err != nil {
		return service.SpeedTestResponse{}, err
	}
	return service.SpeedTestResponse{Parameters: service.ParameterBoundaries{Supported: []string{"provider"}}}, nil
}

func (s *recordingMCPService) AnnotateIPs(_ context.Context, input service.AnnotateIPsRequest) (service.AnnotateIPsResponse, error) {
	if err := s.record("nexttrace_annotate_ips", input); err != nil {
		return service.AnnotateIPsResponse{}, err
	}
	return service.AnnotateIPsResponse{Text: "8.8.8.8 [ok]"}, nil
}

func (s *recordingMCPService) GeoLookup(_ context.Context, input service.GeoLookupRequest) (service.GeoLookupResponse, error) {
	if err := s.record("nexttrace_geo_lookup", input); err != nil {
		return service.GeoLookupResponse{}, err
	}
	return service.GeoLookupResponse{Query: "8.8.8.8"}, nil
}

func (s *recordingMCPService) GlobalpingTrace(_ context.Context, input service.GlobalpingTraceRequest) (service.GlobalpingMeasurementResponse, error) {
	if err := s.record("nexttrace_globalping_trace", input); err != nil {
		return service.GlobalpingMeasurementResponse{}, err
	}
	return service.GlobalpingMeasurementResponse{MeasurementID: "m-1", Status: "finished", ProbesCount: 2}, nil
}

func (s *recordingMCPService) GlobalpingLimits(_ context.Context, input service.GlobalpingLimitsRequest) (service.GlobalpingLimitsResponse, error) {
	if err := s.record("nexttrace_globalping_limits", input); err != nil {
		return service.GlobalpingLimitsResponse{}, err
	}
	return service.GlobalpingLimitsResponse{Measurements: service.GlobalpingMeasurementLimits{Create: service.GlobalpingCreateLimit{Limit: 10, Remaining: 9}}}, nil
}

func (s *recordingMCPService) GlobalpingGetMeasurement(_ context.Context, input service.GlobalpingGetMeasurementRequest) (service.GlobalpingMeasurementResponse, error) {
	if err := s.record("nexttrace_globalping_get_measurement", input); err != nil {
		return service.GlobalpingMeasurementResponse{}, err
	}
	return service.GlobalpingMeasurementResponse{MeasurementID: "m-1", Status: "finished"}, nil
}

func TestMCPHandlerRegistersAllToolsWithSchemas(t *testing.T) {
	session, cleanup := newTestMCPSession(t, newRecordingMCPService())
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	byName := make(map[string]*mcp.Tool)
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		toolCopy := tool
		byName[tool.Name] = toolCopy
		if tool.Description == "" {
			t.Fatalf("tool %s has empty description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Fatalf("tool %s has nil input schema", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Fatalf("tool %s has nil output schema", tool.Name)
		}
	}
	wantNames := []string{
		"nexttrace_capabilities",
		"nexttrace_traceroute",
		"nexttrace_mtr_report",
		"nexttrace_mtr_raw",
		"nexttrace_mtu_trace",
		"nexttrace_speed_test",
		"nexttrace_annotate_ips",
		"nexttrace_geo_lookup",
		"nexttrace_globalping_trace",
		"nexttrace_globalping_limits",
		"nexttrace_globalping_get_measurement",
	}
	sort.Strings(names)
	sort.Strings(wantNames)
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("registered tools = %v, want %v", names, wantNames)
	}
	assertSchemaProperties(t, byName["nexttrace_globalping_trace"].InputSchema, []string{
		"target",
		"locations",
		"limit",
		"protocol",
		"port",
		"packets",
		"ip_version",
	})
}

func TestMCPHandlerCallsEveryToolWithStructuredContent(t *testing.T) {
	svc := newRecordingMCPService()
	session, cleanup := newTestMCPSession(t, svc)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	calls := []struct {
		name          string
		args          map[string]any
		wantInput     any
		wantOutputKey string
	}{
		{
			name:          "nexttrace_capabilities",
			args:          map[string]any{},
			wantInput:     service.CapabilitiesRequest{},
			wantOutputKey: "tools",
		},
		{
			name: "nexttrace_traceroute",
			args: map[string]any{
				"target":        "example.com",
				"protocol":      "tcp",
				"port":          443,
				"queries":       2,
				"source_device": "en8",
			},
			wantInput: service.TraceRequest{
				Target:       "example.com",
				Protocol:     "tcp",
				Port:         443,
				Queries:      2,
				SourceDevice: "en8",
			},
			wantOutputKey: "target",
		},
		{
			name: "nexttrace_mtr_report",
			args: map[string]any{
				"target":          "example.com",
				"max_per_hop":     4,
				"hop_interval_ms": 1000,
			},
			wantInput: service.MTRReportRequest{
				TraceRequest:  service.TraceRequest{Target: "example.com"},
				MaxPerHop:     4,
				HopIntervalMs: 1000,
			},
			wantOutputKey: "stats",
		},
		{
			name: "nexttrace_mtr_raw",
			args: map[string]any{
				"target":      "example.com",
				"duration_ms": 1500,
				"max_per_hop": 3,
			},
			wantInput: service.MTRRawRequest{
				TraceRequest: service.TraceRequest{Target: "example.com"},
				DurationMs:   1500,
				MaxPerHop:    3,
			},
			wantOutputKey: "records",
		},
		{
			name: "nexttrace_mtu_trace",
			args: map[string]any{
				"target":        "example.com",
				"port":          33494,
				"source_device": "en8",
			},
			wantInput: service.MTUTraceRequest{
				Target:       "example.com",
				Port:         33494,
				SourceDevice: "en8",
			},
			wantOutputKey: "path_mtu",
		},
		{
			name: "nexttrace_speed_test",
			args: map[string]any{
				"provider":      "cloudflare",
				"threads":       2,
				"no_metadata":   true,
				"source_device": "en8",
			},
			wantInput: service.SpeedTestRequest{
				Provider:     "cloudflare",
				Threads:      2,
				NoMetadata:   true,
				SourceDevice: "en8",
			},
			wantOutputKey: "result",
		},
		{
			name: "nexttrace_annotate_ips",
			args: map[string]any{
				"text":      "ip 8.8.8.8",
				"ipv4_only": true,
			},
			wantInput:     service.AnnotateIPsRequest{Text: "ip 8.8.8.8", IPv4Only: true},
			wantOutputKey: "text",
		},
		{
			name:          "nexttrace_geo_lookup",
			args:          map[string]any{"query": "8.8.8.8"},
			wantInput:     service.GeoLookupRequest{Query: "8.8.8.8"},
			wantOutputKey: "query",
		},
		{
			name: "nexttrace_globalping_trace",
			args: map[string]any{
				"target":     "w135.gubo.org",
				"locations":  []string{"AS4837"},
				"limit":      2,
				"protocol":   "ICMP",
				"packets":    3,
				"ip_version": 4,
			},
			wantInput: service.GlobalpingTraceRequest{
				Target:    "w135.gubo.org",
				Locations: []string{"AS4837"},
				Limit:     2,
				Protocol:  "ICMP",
				Packets:   3,
				IPVersion: 4,
			},
			wantOutputKey: "measurement_id",
		},
		{
			name:          "nexttrace_globalping_limits",
			args:          map[string]any{},
			wantInput:     service.GlobalpingLimitsRequest{},
			wantOutputKey: "measurements",
		},
		{
			name: "nexttrace_globalping_get_measurement",
			args: map[string]any{
				"measurement_id": "m-1",
			},
			wantInput:     service.GlobalpingGetMeasurementRequest{MeasurementID: "m-1"},
			wantOutputKey: "measurement_id",
		},
	}

	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
			if err != nil {
				t.Fatalf("CallTool returned error: %v", err)
			}
			if result.IsError {
				t.Fatalf("CallTool returned tool error: %#v", result.Content)
			}
			if len(result.Content) == 0 {
				t.Fatal("CallTool returned no content")
			}
			structured := structuredContentMap(t, result)
			if _, ok := structured[call.wantOutputKey]; !ok {
				t.Fatalf("structuredContent missing %q: %#v", call.wantOutputKey, structured)
			}
			if svc.calls[call.name] != 1 {
				t.Fatalf("service call count for %s = %d, want 1", call.name, svc.calls[call.name])
			}
			if !reflect.DeepEqual(svc.inputs[call.name], call.wantInput) {
				t.Fatalf("service input for %s = %#v, want %#v", call.name, svc.inputs[call.name], call.wantInput)
			}
		})
	}
}

func TestMCPHandlerReturnsServiceErrorsAsToolErrors(t *testing.T) {
	svc := newRecordingMCPService()
	svc.failTool = "nexttrace_geo_lookup"
	session, cleanup := newTestMCPSession(t, svc)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "nexttrace_geo_lookup",
		Arguments: map[string]any{"query": "8.8.8.8"},
	})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("CallTool IsError = false, content=%#v", result.Content)
	}
	if !strings.Contains(toolText(result), "forced nexttrace_geo_lookup") {
		t.Fatalf("tool error content = %q", toolText(result))
	}
}

func newTestMCPSession(t *testing.T, svc nexttraceMCPService) (*mcp.ClientSession, func()) {
	t.Helper()

	ts := httptest.NewServer(newMCPHTTPHandlerWithService(svc))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	connectOK := false
	defer func() {
		if !connectOK {
			cancel()
			ts.Close()
		}
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	connectOK = true

	return session, func() {
		session.Close()
		cancel()
		ts.Close()
	}
}

func structuredContentMap(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	if result.StructuredContent == nil {
		t.Fatal("structuredContent is nil")
	}
	var payload []byte
	switch structured := result.StructuredContent.(type) {
	case json.RawMessage:
		payload = structured
	default:
		var err error
		payload, err = json.Marshal(structured)
		if err != nil {
			t.Fatalf("marshal structuredContent: %v", err)
		}
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal structuredContent %s: %v", payload, err)
	}
	return out
}

func schemaMap(t *testing.T, schema any) map[string]any {
	t.Helper()

	payload, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal schema %s: %v", payload, err)
	}
	return out
}

func assertSchemaProperties(t *testing.T, schema any, names []string) {
	t.Helper()

	decoded := schemaMap(t, schema)
	properties, ok := decoded["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema missing properties: %#v", decoded)
	}
	for _, name := range names {
		if _, ok := properties[name]; !ok {
			t.Fatalf("schema missing property %s: %#v", name, properties)
		}
	}
}

func toolText(result *mcp.CallToolResult) string {
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}
