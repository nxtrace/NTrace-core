package server

import (
	"context"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/internal/service"
)

type nexttraceMCPService interface {
	Capabilities(context.Context, service.CapabilitiesRequest) (service.CapabilitiesResponse, error)
	Traceroute(context.Context, service.TraceRequest) (service.TraceResponse, error)
	MTRReport(context.Context, service.MTRReportRequest) (service.MTRReportResponse, error)
	MTRRaw(context.Context, service.MTRRawRequest) (service.MTRRawResponse, error)
	MTUTrace(context.Context, service.MTUTraceRequest) (service.MTUTraceResponse, error)
	SpeedTest(context.Context, service.SpeedTestRequest) (service.SpeedTestResponse, error)
	AnnotateIPs(context.Context, service.AnnotateIPsRequest) (service.AnnotateIPsResponse, error)
	GeoLookup(context.Context, service.GeoLookupRequest) (service.GeoLookupResponse, error)
	GlobalpingTrace(context.Context, service.GlobalpingTraceRequest) (service.GlobalpingMeasurementResponse, error)
	GlobalpingLimits(context.Context, service.GlobalpingLimitsRequest) (service.GlobalpingLimitsResponse, error)
	GlobalpingGetMeasurement(context.Context, service.GlobalpingGetMeasurementRequest) (service.GlobalpingMeasurementResponse, error)
}

func newMCPHTTPHandler() http.Handler {
	return newMCPHTTPHandlerWithService(service.New())
}

func newMCPHTTPHandlerWithService(svc nexttraceMCPService) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "nexttrace",
		Title:   "NextTrace Deploy MCP",
		Version: config.Version,
	}, &mcp.ServerOptions{
		Instructions: "Use NextTrace tools for local traceroute, MTR, MTU, speed, IP annotation, GeoIP lookup, and Globalping multi-location traceroute.",
	})
	registerMCPTools(server, svc)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:      true,
		JSONResponse:   true,
		SessionTimeout: 5 * time.Minute,
	})
}

func registerMCPTools(server *mcp.Server, svc nexttraceMCPService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_capabilities",
		Description: "List NextTrace MCP tools and parameter support boundaries.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.CapabilitiesRequest) (*mcp.CallToolResult, service.CapabilitiesResponse, error) {
		out, err := svc.Capabilities(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_traceroute",
		Description: "Run local NextTrace ICMP/TCP/UDP traceroute and return structured hop attempts.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.TraceRequest) (*mcp.CallToolResult, service.TraceResponse, error) {
		out, err := svc.Traceroute(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_mtr_report",
		Description: "Run bounded local MTR report and return per-hop latency/loss statistics.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.MTRReportRequest) (*mcp.CallToolResult, service.MTRReportResponse, error) {
		out, err := svc.MTRReport(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_mtr_raw",
		Description: "Run bounded local MTR raw mode and return probe-level stream records.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.MTRRawRequest) (*mcp.CallToolResult, service.MTRRawResponse, error) {
		out, err := svc.MTRRaw(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_mtu_trace",
		Description: "Run UDP path-MTU discovery and return structured MTU hops.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.MTUTraceRequest) (*mcp.CallToolResult, service.MTUTraceResponse, error) {
		out, err := svc.MTUTrace(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_speed_test",
		Description: "Run a conservative local speed test and return JSON result fields.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.SpeedTestRequest) (*mcp.CallToolResult, service.SpeedTestResponse, error) {
		out, err := svc.SpeedTest(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_annotate_ips",
		Description: "Annotate IP literals in text with NextTrace GeoIP metadata.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.AnnotateIPsRequest) (*mcp.CallToolResult, service.AnnotateIPsResponse, error) {
		out, err := svc.AnnotateIPs(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_geo_lookup",
		Description: "Look up GeoIP metadata for one IPv4 or IPv6 address.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.GeoLookupRequest) (*mcp.CallToolResult, service.GeoLookupResponse, error) {
		out, err := svc.GeoLookup(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_globalping_trace",
		Description: "Run a Globalping multi-location MTR/traceroute measurement and return all probe results.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.GlobalpingTraceRequest) (*mcp.CallToolResult, service.GlobalpingMeasurementResponse, error) {
		out, err := svc.GlobalpingTrace(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_globalping_limits",
		Description: "Return current Globalping rate and credit limits.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.GlobalpingLimitsRequest) (*mcp.CallToolResult, service.GlobalpingLimitsResponse, error) {
		out, err := svc.GlobalpingLimits(ctx, input)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nexttrace_globalping_get_measurement",
		Description: "Fetch a previous Globalping measurement by measurement_id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input service.GlobalpingGetMeasurementRequest) (*mcp.CallToolResult, service.GlobalpingMeasurementResponse, error) {
		out, err := svc.GlobalpingGetMeasurement(ctx, input)
		return nil, out, err
	})
}
