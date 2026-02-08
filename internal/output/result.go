package output

import (
	"probeHTTP/internal/hash"
)

// TLSInfo provides httpx-compatible nested TLS information
type TLSInfo struct {
	Version string `json:"version,omitempty"`
	Cipher  string `json:"cipher,omitempty"`
}

// ProbeResult represents the JSON output for each probed URL
type ProbeResult struct {
	Timestamp        string   `json:"timestamp"`
	Hash             hash.Hash `json:"hash"`
	Port             string   `json:"port"`
	URL              string   `json:"url"`
	Input            string   `json:"input"`
	FinalURL         string   `json:"final_url"`
	Title            string   `json:"title"`
	Scheme           string   `json:"scheme"`
	WebServer        string   `json:"webserver"`
	ContentType      string   `json:"content_type"`
	Method           string   `json:"method"`
	Host             string   `json:"host"`
	IP               string   `json:"a,omitempty"`
	HostIP           string   `json:"host_ip,omitempty"`
	Path             string   `json:"path"`
	Time             string   `json:"time"`
	ChainStatusCodes []int    `json:"chain_status_codes"`
	ChainHosts       []string `json:"chain_hosts"`
	Words            int      `json:"words"`
	Lines            int      `json:"lines"`
	StatusCode       int      `json:"status_code"`
	ContentLength    int      `json:"content_length"`
	TLSVersion       string   `json:"tls_version,omitempty"`
	CipherSuite      string   `json:"cipher_suite,omitempty"`
	Protocol         string   `json:"protocol,omitempty"`
	TLSConfigStrategy string  `json:"tls_config_strategy,omitempty"`
	HSTS             bool     `json:"hsts,omitempty"`
	HSTSHeader       string   `json:"hsts_header,omitempty"`
	TLS              *TLSInfo `json:"tls,omitempty"`
	Technologies     []string `json:"tech,omitempty"`
	CDN              bool     `json:"cdn,omitempty"`
	CDNName          string   `json:"cdn_name,omitempty"`
	CNAME            string   `json:"cname,omitempty"`
	Error            string   `json:"error,omitempty"`
	// Storage-related fields (optional, based on flags)
	ResponseHeaders    map[string]string `json:"response_headers,omitempty"`
	RequestHeaders     map[string]string `json:"request_headers,omitempty"`
	RawRequest         string            `json:"raw_request,omitempty"`
	RawResponse        string            `json:"raw_response,omitempty"`
	StoredResponsePath string            `json:"stored_response_path,omitempty"`
}
