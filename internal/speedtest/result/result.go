package result

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type RunConfig struct {
	Provider       string `json:"provider"`
	Max            string `json:"max"`
	MaxBytes       int64  `json:"max_bytes"`
	TimeoutMs      int    `json:"timeout_ms"`
	Threads        int    `json:"threads"`
	LatencyCount   int    `json:"latency_count"`
	JSON           bool   `json:"json"`
	NonInteractive bool   `json:"non_interactive"`
	EndpointIP     string `json:"endpoint_ip,omitempty"`
	Metadata       bool   `json:"metadata"`
	Language       string `json:"language"`
	NoColor        bool   `json:"no_color"`
	DotServer      string `json:"dot_server,omitempty"`
	Source         string `json:"source,omitempty"`
	Device         string `json:"device,omitempty"`
}

type CandidateResult struct {
	IP          string   `json:"ip"`
	Description string   `json:"description,omitempty"`
	RTTMs       *float64 `json:"rtt_ms,omitempty"`
	Source      string   `json:"source,omitempty"`
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
}

type SelectedEndpoint struct {
	IP          string   `json:"ip,omitempty"`
	Description string   `json:"description,omitempty"`
	RTTMs       *float64 `json:"rtt_ms,omitempty"`
	Source      string   `json:"source,omitempty"`
	Status      string   `json:"status"`
}

type PeerInfo struct {
	Status       string         `json:"status"`
	IP           string         `json:"ip,omitempty"`
	ISP          string         `json:"isp,omitempty"`
	ASN          string         `json:"asn,omitempty"`
	Location     string         `json:"location,omitempty"`
	ProviderMeta map[string]any `json:"provider_meta,omitempty"`
}

type ConnectionInfo struct {
	Status          string   `json:"status"`
	MetadataEnabled bool     `json:"metadata_enabled"`
	Host            string   `json:"host,omitempty"`
	Client          PeerInfo `json:"client"`
	Server          PeerInfo `json:"server"`
}

type LatencyResult struct {
	Status   string   `json:"status"`
	Samples  int      `json:"samples"`
	MinMs    *float64 `json:"min_ms,omitempty"`
	AvgMs    *float64 `json:"avg_ms,omitempty"`
	MedianMs *float64 `json:"median_ms,omitempty"`
	MaxMs    *float64 `json:"max_ms,omitempty"`
	JitterMs *float64 `json:"jitter_ms,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type RoundResult struct {
	Name          string        `json:"name"`
	Direction     string        `json:"direction"`
	Threads       int           `json:"threads"`
	Status        string        `json:"status"`
	URL           string        `json:"url"`
	TotalBytes    int64         `json:"total_bytes"`
	DurationMs    int64         `json:"duration_ms"`
	Mbps          float64       `json:"mbps"`
	FaultCount    int           `json:"fault_count"`
	HadFault      bool          `json:"had_fault"`
	LoadedLatency LatencyResult `json:"loaded_latency"`
	Error         string        `json:"error,omitempty"`
}

type RunResult struct {
	SchemaVersion    int               `json:"schema_version"`
	Config           RunConfig         `json:"config"`
	Candidates       []CandidateResult `json:"candidates"`
	SelectedEndpoint SelectedEndpoint  `json:"selected_endpoint"`
	ConnectionInfo   ConnectionInfo    `json:"connection_info"`
	IdleLatency      LatencyResult     `json:"idle_latency"`
	Rounds           []RoundResult     `json:"rounds"`
	TotalBytes       int64             `json:"total_bytes"`
	Warnings         []Warning         `json:"warnings"`
	Degraded         bool              `json:"degraded"`
	ExitCode         int               `json:"exit_code"`
	StartedAt        string            `json:"started_at"`
	DurationMs       int64             `json:"duration_ms"`
}
