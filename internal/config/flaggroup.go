package config

import (
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
)

// FlagType represents the type of a flag value
type FlagType int

const (
	BoolType FlagType = iota
	StringType
	IntType
)

// FlagDef holds metadata for a single flag (short + long names, type, default, description)
type FlagDef struct {
	Short       string
	Long        string
	Type        FlagType
	Default     interface{}
	Description string
}

// FlagGroup is a named category containing related flags
type FlagGroup struct {
	Name  string
	Flags []FlagDef
}

// HelpFormatter holds the tool info and ordered flag groups for custom help rendering
type HelpFormatter struct {
	ToolName    string
	Description string
	Groups      []*FlagGroup
}

// addBoolFlag registers a bool flag with both short and long names and appends it to the group
func addBoolFlag(group *FlagGroup, p *bool, short, long string, value bool, usage string) {
	if short != "" {
		flag.BoolVar(p, short, value, usage)
	}
	if long != "" {
		flag.BoolVar(p, long, value, usage)
	}
	group.Flags = append(group.Flags, FlagDef{
		Short:       short,
		Long:        long,
		Type:        BoolType,
		Default:     value,
		Description: usage,
	})
}

// addStringFlag registers a string flag with both short and long names and appends it to the group
func addStringFlag(group *FlagGroup, p *string, short, long string, value string, usage string) {
	if short != "" {
		flag.StringVar(p, short, value, usage)
	}
	if long != "" {
		flag.StringVar(p, long, value, usage)
	}
	group.Flags = append(group.Flags, FlagDef{
		Short:       short,
		Long:        long,
		Type:        StringType,
		Default:     value,
		Description: usage,
	})
}

// addIntFlag registers an int flag with both short and long names and appends it to the group
func addIntFlag(group *FlagGroup, p *int, short, long string, value int, usage string) {
	if short != "" {
		flag.IntVar(p, short, value, usage)
	}
	if long != "" {
		flag.IntVar(p, long, value, usage)
	}
	group.Flags = append(group.Flags, FlagDef{
		Short:       short,
		Long:        long,
		Type:        IntType,
		Default:     value,
		Description: usage,
	})
}

// RegisterFlags creates all flag groups, registers every flag with the standard flag package,
// and returns a populated HelpFormatter.
func RegisterFlags(cfg *Config) *HelpFormatter {
	formatter := &HelpFormatter{
		ToolName:    "probeHTTP",
		Description: "fast HTTP probing tool",
	}

	// INPUT
	input := &FlagGroup{Name: "INPUT"}
	addStringFlag(input, &cfg.InputFile, "i", "input", "", "Input file (default: stdin)")
	formatter.Groups = append(formatter.Groups, input)

	// OUTPUT
	output := &FlagGroup{Name: "OUTPUT"}
	addStringFlag(output, &cfg.OutputFile, "o", "output", "", "Output file (default: stdout)")
	addBoolFlag(output, &cfg.StoreResponse, "sr", "store-response", false, "Store HTTP responses to output directory")
	addStringFlag(output, &cfg.StoreResponseDir, "srd", "store-response-dir", "output", "Directory to store HTTP responses")
	addBoolFlag(output, &cfg.IncludeResponseHeader, "irh", "include-response-header", false, "Include response headers in JSON output")
	addBoolFlag(output, &cfg.IncludeResponse, "irr", "include-response", false, "Include full request/response in JSON output")
	formatter.Groups = append(formatter.Groups, output)

	// PROBES
	probes := &FlagGroup{Name: "PROBES"}
	addBoolFlag(probes, &cfg.ResolveIP, "rip", "resolve-ip", false, "Resolve and include IP address in output")
	addBoolFlag(probes, &cfg.DetectHSTS, "hsts", "detect-hsts", false, "Detect and report HSTS headers")
	addBoolFlag(probes, &cfg.TechDetect, "td", "tech-detect", false, "Enable technology detection using wappalyzer")
	addBoolFlag(probes, &cfg.DetectCDN, "cdn", "detect-cdn", false, "Detect CDN from response headers")
	addBoolFlag(probes, &cfg.DetectCNAME, "cname", "detect-cname", false, "Resolve and report CNAME records")
	addBoolFlag(probes, &cfg.ExtractTLS, "xtls", "extract-tls", false, "Extract TLS certificate details (subject, SANs, issuer, validity)")
	addBoolFlag(probes, &cfg.ExtractTLSChain, "", "extract-tls-chain", false, "Include intermediate certificate chain (implies --extract-tls)")
	addBoolFlag(probes, &cfg.DiscoverDomains, "dd", "discover-domains", false, "Discover domains from certificate SANs/CN and CSP headers")
	formatter.Groups = append(formatter.Groups, probes)

	// CONFIGURATION
	configuration := &FlagGroup{Name: "CONFIGURATION"}
	addBoolFlag(configuration, &cfg.FollowRedirects, "fr", "follow-redirects", true, "Follow redirects")
	addIntFlag(configuration, &cfg.MaxRedirects, "maxr", "max-redirects", 10, "Max redirects")
	addBoolFlag(configuration, &cfg.SameHostOnly, "sho", "same-host-only", false, "Only follow redirects to same hostname")
	addBoolFlag(configuration, &cfg.AllSchemes, "as", "all-schemes", false, "Test both HTTP and HTTPS schemes")
	addBoolFlag(configuration, &cfg.IgnorePorts, "ip", "ignore-ports", false, "Ignore input ports and test common HTTP/HTTPS ports")
	addStringFlag(configuration, &cfg.CustomPorts, "p", "ports", "", "Custom port list (comma-separated, supports ranges)")
	addBoolFlag(configuration, &cfg.InsecureSkipVerify, "k", "insecure", false, "Skip TLS certificate verification")
	addBoolFlag(configuration, &cfg.AllowPrivateIPs, "", "allow-private", false, "Allow scanning private IP addresses")
	addStringFlag(configuration, &cfg.UserAgent, "ua", "user-agent", "", "Custom User-Agent header")
	addBoolFlag(configuration, &cfg.RandomUserAgent, "rua", "random-user-agent", false, "Use random User-Agent from pool")
	addBoolFlag(configuration, &cfg.DisableHTTP3, "", "disable-http3", false, "Disable HTTP/3 (QUIC) support")
	formatter.Groups = append(formatter.Groups, configuration)

	// RATE-LIMIT
	rateLimit := &FlagGroup{Name: "RATE-LIMIT"}
	addIntFlag(rateLimit, &cfg.Timeout, "t", "timeout", 10, "Request timeout in seconds")
	addIntFlag(rateLimit, &cfg.Concurrency, "c", "concurrency", 20, "Concurrent requests")
	addIntFlag(rateLimit, &cfg.TLSHandshakeTimeout, "tls-timeout", "tls-handshake-timeout", 10, "TLS handshake timeout in seconds")
	addIntFlag(rateLimit, &cfg.RateLimitTimeout, "", "rate-limit-timeout", 60, "Rate limit wait timeout in seconds")
	addIntFlag(rateLimit, &cfg.MaxRetries, "", "retries", 0, "Maximum number of retries for failed requests")
	formatter.Groups = append(formatter.Groups, rateLimit)

	// DEBUG
	debug := &FlagGroup{Name: "DEBUG"}
	addBoolFlag(debug, &cfg.Debug, "d", "debug", false, "Debug mode (show all requests and responses to stderr)")
	addBoolFlag(debug, &cfg.Silent, "", "silent", false, "Silent mode (no errors to stderr)")
	addStringFlag(debug, &cfg.DebugLogFile, "", "debug-log", "", "Write detailed debug logs to file")
	formatter.Groups = append(formatter.Groups, debug)

	// MISCELLANEOUS
	misc := &FlagGroup{Name: "MISCELLANEOUS"}
	addBoolFlag(misc, &cfg.Version, "v", "version", false, "Show version information")
	formatter.Groups = append(formatter.Groups, misc)

	return formatter
}

// PrintUsage writes the grouped help output to w
func (h *HelpFormatter) PrintUsage(w io.Writer) {
	fmt.Fprintf(w, "%s - %s\n\n", h.ToolName, h.Description)
	fmt.Fprintf(w, "Usage:\n  %s [flags]\n\nFlags:\n", h.ToolName)

	for _, group := range h.Groups {
		fmt.Fprintf(w, "\n%s:\n", group.Name)

		tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		for _, f := range group.Flags {
			name := formatFlagName(f)
			typeSuffix := formatFlagType(f)
			defaultStr := formatFlagDefault(f)

			desc := f.Description
			if defaultStr != "" {
				desc += " " + defaultStr
			}

			fmt.Fprintf(tw, "   %s%s\t%s\n", name, typeSuffix, desc)
		}
		tw.Flush()
	}
}

// formatFlagName builds the "-short, -long" or just "-long" name string
func formatFlagName(f FlagDef) string {
	if f.Short != "" && f.Long != "" {
		return fmt.Sprintf("-%s, -%s", f.Short, f.Long)
	}
	if f.Short != "" {
		return fmt.Sprintf("-%s", f.Short)
	}
	return fmt.Sprintf("-%s", f.Long)
}

// formatFlagType returns the type suffix for non-bool flags
func formatFlagType(f FlagDef) string {
	switch f.Type {
	case StringType:
		return " string"
	case IntType:
		return " int"
	default:
		return ""
	}
}

// formatFlagDefault returns a parenthesized default value string for non-zero defaults
func formatFlagDefault(f FlagDef) string {
	switch f.Type {
	case BoolType:
		if v, ok := f.Default.(bool); ok && v {
			return "(default true)"
		}
	case IntType:
		if v, ok := f.Default.(int); ok && v != 0 {
			return fmt.Sprintf("(default %d)", v)
		}
	case StringType:
		if v, ok := f.Default.(string); ok && v != "" {
			return fmt.Sprintf("(default %q)", v)
		}
	}
	return ""
}
