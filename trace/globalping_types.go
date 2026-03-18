package trace

// GlobalpingOptions configures a Globalping-based traceroute request.
// Defined in an untagged file so all build flavors can reference the type
// without pulling in the globalping-cli dependency.
type GlobalpingOptions struct {
	Target  string
	From    string
	IPv4    bool
	IPv6    bool
	TCP     bool
	UDP     bool
	Port    int
	Packets int
	MaxHops int

	DisableMaptrace bool
	DataOrigin      string

	TablePrint   bool
	ClearScreen  bool
	ClassicPrint bool
	RawPrint     bool
	JSONPrint    bool
}
