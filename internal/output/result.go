package output

import (
	"probeHTTP/internal/hash"
)

// CertificateInfo holds parsed X.509 certificate details.
type CertificateInfo struct {
	SubjectCN    string   `json:"subject_cn,omitempty"`
	SubjectOrg   string   `json:"subject_org,omitempty"`
	IssuerCN     string   `json:"issuer_cn,omitempty"`
	IssuerOrg    string   `json:"issuer_org,omitempty"`
	SANs         []string `json:"sans,omitempty"`
	NotBefore    string   `json:"not_before,omitempty"`
	NotAfter     string   `json:"not_after,omitempty"`
	SerialNumber string   `json:"serial_number,omitempty"`
	Fingerprint  string   `json:"fingerprint_sha256,omitempty"`
	IsExpired    bool     `json:"is_expired,omitempty"`
	IsSelfSigned bool     `json:"is_self_signed,omitempty"`
	KeyAlgorithm string   `json:"key_algorithm,omitempty"`
	KeySize      int      `json:"key_size,omitempty"`
	SigAlgorithm string   `json:"sig_algorithm,omitempty"`
}

// TLSInfo provides nested TLS connection and certificate information.
type TLSInfo struct {
	Version     string            `json:"version,omitempty"`
	Cipher      string            `json:"cipher,omitempty"`
	Certificate *CertificateInfo  `json:"certificate,omitempty"`
	Chain       []CertificateInfo `json:"chain,omitempty"`
}

// DiscoveredDomains holds domains found via TLS certificates and CSP headers.
type DiscoveredDomains struct {
	Domains       []string          `json:"domains,omitempty"`
	DomainSources map[string]string `json:"domain_sources,omitempty"`
	NewDomains    []string          `json:"new_domains,omitempty"`
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
	SNIRequired      bool     `json:"sni_required,omitempty"`
	Diagnostic       string   `json:"diagnostic,omitempty"`
	// TLS extraction fields (optional, enabled via --extract-tls)
	DiscoveredDomains *DiscoveredDomains `json:"discovered_domains,omitempty"`
	// Storage-related fields (optional, based on flags)
	ResponseHeaders    map[string]string `json:"response_headers,omitempty"`
	RequestHeaders     map[string]string `json:"request_headers,omitempty"`
	RawRequest         string            `json:"raw_request,omitempty"`
	RawResponse        string            `json:"raw_response,omitempty"`
	StoredResponsePath string            `json:"stored_response_path,omitempty"`
}
