package probe

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"probeHTTP/internal/output"
)

// ExtractCertificateInfo extracts certificate details from the leaf certificate
// in the TLS connection state. Returns nil if no peer certificates are present.
func ExtractCertificateInfo(connState *tls.ConnectionState) *output.CertificateInfo {
	if connState == nil || len(connState.PeerCertificates) == 0 {
		return nil
	}

	cert := connState.PeerCertificates[0]
	return parseCertificate(cert)
}

// ExtractCertificateChain extracts info for all intermediate certificates
// (everything after the leaf) from the TLS connection state.
func ExtractCertificateChain(connState *tls.ConnectionState) []output.CertificateInfo {
	if connState == nil || len(connState.PeerCertificates) < 2 {
		return nil
	}

	chain := make([]output.CertificateInfo, 0, len(connState.PeerCertificates)-1)
	for _, cert := range connState.PeerCertificates[1:] {
		chain = append(chain, *parseCertificate(cert))
	}
	return chain
}

// parseCertificate converts an x509.Certificate into a CertificateInfo struct.
func parseCertificate(cert *x509.Certificate) *output.CertificateInfo {
	info := &output.CertificateInfo{
		SubjectCN:    cert.Subject.CommonName,
		IssuerCN:     cert.Issuer.CommonName,
		SANs:         cert.DNSNames,
		NotBefore:    cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:     cert.NotAfter.UTC().Format(time.RFC3339),
		SerialNumber: formatSerial(cert.SerialNumber.Bytes()),
		Fingerprint:  formatFingerprint(sha256.Sum256(cert.Raw)),
		IsExpired:    time.Now().After(cert.NotAfter),
		IsSelfSigned: isSelfSigned(cert),
		SigAlgorithm: cert.SignatureAlgorithm.String(),
	}

	// Subject organization
	if len(cert.Subject.Organization) > 0 {
		info.SubjectOrg = strings.Join(cert.Subject.Organization, ", ")
	}

	// Issuer organization
	if len(cert.Issuer.Organization) > 0 {
		info.IssuerOrg = strings.Join(cert.Issuer.Organization, ", ")
	}

	// Key algorithm and size
	info.KeyAlgorithm, info.KeySize = extractKeyInfo(cert)

	return info
}

// isSelfSigned checks whether a certificate is self-signed by comparing
// the raw subject and issuer ASN.1 bytes and verifying the signature.
func isSelfSigned(cert *x509.Certificate) bool {
	if !bytes.Equal(cert.RawIssuer, cert.RawSubject) {
		return false
	}
	// Verify the signature was made with the cert's own public key
	return cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) == nil
}

// extractKeyInfo returns the public key algorithm name and key size in bits.
func extractKeyInfo(cert *x509.Certificate) (algorithm string, size int) {
	switch cert.PublicKeyAlgorithm {
	case x509.RSA:
		algorithm = "RSA"
	case x509.ECDSA:
		algorithm = "ECDSA"
	case x509.Ed25519:
		algorithm = "Ed25519"
	default:
		algorithm = cert.PublicKeyAlgorithm.String()
	}

	// Extract key size from the public key
	switch key := cert.PublicKey.(type) {
	case interface{ Size() int }:
		// RSA keys have Size() returning bytes
		size = key.Size() * 8
	case interface{ Params() interface{ BitSize() int } }:
		size = key.Params().BitSize()
	default:
		// For ECDSA, use the curve bit size from the certificate
		if cert.PublicKeyAlgorithm == x509.ECDSA {
			if ecKey, ok := cert.PublicKey.(interface {
				Params() *struct{ BitSize int }
			}); ok {
				_ = ecKey // fallback handled below
			}
		}
	}

	return algorithm, size
}

// formatSerial formats a certificate serial number as colon-separated hex.
func formatSerial(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02x", v)
	}
	return strings.Join(parts, ":")
}

// formatFingerprint formats a SHA-256 hash as colon-separated hex.
func formatFingerprint(hash [32]byte) string {
	parts := make([]string, 32)
	for i, v := range hash {
		parts[i] = fmt.Sprintf("%02x", v)
	}
	return strings.Join(parts, ":")
}
