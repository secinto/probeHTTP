package tech

import (
	"net/http"

	wappalyzer "github.com/projectdiscovery/wappalyzergo"
)

// Detector wraps wappalyzergo for technology detection
type Detector struct {
	wappalyze *wappalyzer.Wappalyze
}

// NewDetector creates a new technology detector
func NewDetector() (*Detector, error) {
	wappalyze, err := wappalyzer.New()
	if err != nil {
		return nil, err
	}
	return &Detector{wappalyze: wappalyze}, nil
}

// Detect identifies technologies from HTTP headers and body
func (d *Detector) Detect(headers http.Header, body []byte) []string {
	if d == nil || d.wappalyze == nil {
		return nil
	}

	fingerprints := d.wappalyze.Fingerprint(headers, body)
	if len(fingerprints) == 0 {
		return nil
	}

	techs := make([]string, 0, len(fingerprints))
	for tech := range fingerprints {
		techs = append(techs, tech)
	}
	return techs
}
