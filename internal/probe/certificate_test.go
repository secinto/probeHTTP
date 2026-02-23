package probe

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

// newSelfSignedCert creates a self-signed certificate for testing.
func newSelfSignedCert(t *testing.T, cn string, sans []string, notBefore, notAfter time.Time) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"Test Corp"}},
		Issuer:       pkix.Name{CommonName: cn, Organization: []string{"Test Corp"}},
		DNSNames:     sans,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	return cert, key
}

func TestExtractCertificateInfo_NilConnState(t *testing.T) {
	info := ExtractCertificateInfo(nil)
	if info != nil {
		t.Fatal("expected nil for nil connection state")
	}
}

func TestExtractCertificateInfo_NoPeerCerts(t *testing.T) {
	info := ExtractCertificateInfo(&tls.ConnectionState{})
	if info != nil {
		t.Fatal("expected nil for empty peer certificates")
	}
}

func TestExtractCertificateInfo_ValidCert(t *testing.T) {
	now := time.Now()
	cert, _ := newSelfSignedCert(t, "example.com",
		[]string{"example.com", "www.example.com", "api.example.com"},
		now.Add(-24*time.Hour), now.Add(365*24*time.Hour))

	connState := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	info := ExtractCertificateInfo(connState)
	if info == nil {
		t.Fatal("expected non-nil CertificateInfo")
	}

	if info.SubjectCN != "example.com" {
		t.Errorf("SubjectCN = %q, want %q", info.SubjectCN, "example.com")
	}

	if info.SubjectOrg != "Test Corp" {
		t.Errorf("SubjectOrg = %q, want %q", info.SubjectOrg, "Test Corp")
	}

	if info.IssuerCN != "example.com" {
		t.Errorf("IssuerCN = %q, want %q", info.IssuerCN, "example.com")
	}

	if len(info.SANs) != 3 {
		t.Errorf("SANs count = %d, want 3", len(info.SANs))
	}

	if info.IsExpired {
		t.Error("expected cert not to be expired")
	}

	if !info.IsSelfSigned {
		t.Error("expected self-signed cert to be detected")
	}

	if info.KeyAlgorithm != "ECDSA" {
		t.Errorf("KeyAlgorithm = %q, want %q", info.KeyAlgorithm, "ECDSA")
	}

	if info.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}

	if info.SerialNumber == "" {
		t.Error("expected non-empty serial number")
	}
}

func TestExtractCertificateInfo_ExpiredCert(t *testing.T) {
	now := time.Now()
	cert, _ := newSelfSignedCert(t, "expired.com", nil,
		now.Add(-365*24*time.Hour), now.Add(-24*time.Hour))

	connState := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	info := ExtractCertificateInfo(connState)
	if info == nil {
		t.Fatal("expected non-nil CertificateInfo")
	}

	if !info.IsExpired {
		t.Error("expected cert to be detected as expired")
	}
}

func TestExtractCertificateChain_NoChain(t *testing.T) {
	now := time.Now()
	cert, _ := newSelfSignedCert(t, "leaf.com", nil,
		now.Add(-24*time.Hour), now.Add(365*24*time.Hour))

	connState := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	chain := ExtractCertificateChain(connState)
	if chain != nil {
		t.Fatal("expected nil chain for single cert")
	}
}

func TestExtractCertificateChain_WithIntermediate(t *testing.T) {
	now := time.Now()
	leaf, _ := newSelfSignedCert(t, "leaf.com", []string{"leaf.com"},
		now.Add(-24*time.Hour), now.Add(365*24*time.Hour))
	intermediate, _ := newSelfSignedCert(t, "Intermediate CA", nil,
		now.Add(-365*24*time.Hour), now.Add(10*365*24*time.Hour))

	connState := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf, intermediate},
	}

	chain := ExtractCertificateChain(connState)
	if len(chain) != 1 {
		t.Fatalf("chain length = %d, want 1", len(chain))
	}

	if chain[0].SubjectCN != "Intermediate CA" {
		t.Errorf("chain[0].SubjectCN = %q, want %q", chain[0].SubjectCN, "Intermediate CA")
	}
}

func TestFormatSerial(t *testing.T) {
	result := formatSerial([]byte{0x04, 0x1e, 0xab})
	if result != "04:1e:ab" {
		t.Errorf("formatSerial = %q, want %q", result, "04:1e:ab")
	}
}

func TestFormatFingerprint(t *testing.T) {
	var hash [32]byte
	hash[0] = 0xab
	hash[1] = 0xcd
	result := formatFingerprint(hash)
	if len(result) != 32*3-1 { // 32 hex pairs with 31 colons
		t.Errorf("fingerprint length = %d, want %d", len(result), 32*3-1)
	}
}
