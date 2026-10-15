package nas

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"dwc-server-advanced/common"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateHTTPSCertificateSets(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "fullchain.pem"), filepath.Join(dir, "privkey.pem")
	if err := generateNormalCertificate(certPath, keyPath); err != nil {
		t.Fatal(err)
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatalf("normal HTTPS pair: %v", err)
	}
	if err := generateNormalCertificate(certPath, keyPath); err != nil {
		t.Fatalf("existing pair should be preserved: %v", err)
	}
	if after, err := os.ReadFile(keyPath); err != nil || string(after) != string(keyPEM) {
		t.Fatal("existing HTTPS private key was replaced")
	}

	// A non-CA client certificate mirrors Dolphin's certificate used by the
	// legacy Wii/DS chain. Generation must work with that signing key.
	parentKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	parentTemplate := certificateTemplate("Wii client")
	parentTemplate.IsCA = false
	parentDER, err := x509.CreateCertificate(rand.Reader, &parentTemplate, &parentTemplate, &parentKey.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := x509.ParseCertificate(parentDER)
	if err != nil {
		t.Fatal(err)
	}

	wiiCert, wiiKey := filepath.Join(dir, "naswii-chain.der"), filepath.Join(dir, "naswii-signed-key.pem")
	if err := generateExploitCertificate("naswii.nintendowifi.net", parent, parentKey, wiiCert, wiiKey, "", false); err != nil {
		t.Fatal(err)
	}
	chain, err := os.ReadFile(wiiCert)
	if err != nil {
		t.Fatal(err)
	}
	certs, err := x509.ParseCertificates(chain)
	if err != nil || len(certs) != 2 || certs[0].Subject.CommonName != "naswii.nintendowifi.net" {
		t.Fatalf("Wii DER chain: %v, count %d", err, len(certs))
	}
	if !validPKCS8RSAKey(t, wiiKey) {
		t.Fatal("Wii key is not PKCS8 RSA")
	}

	dsCert, dsKey, intermediate := filepath.Join(dir, "nas-cert.der"), filepath.Join(dir, "nas-key.pem"), filepath.Join(dir, "nwc.der")
	if err := generateExploitCertificate("nas.nintendowifi.net", parent, parentKey, dsCert, dsKey, intermediate, true); err != nil {
		t.Fatal(err)
	}
	dsDER, err := os.ReadFile(dsCert)
	if err != nil {
		t.Fatal(err)
	}
	dsParsed, err := x509.ParseCertificate(dsDER)
	if err != nil || dsParsed.Subject.CommonName != "nas.nintendowifi.net" {
		t.Fatalf("DS DER certificate: %v", err)
	}
	if !validPKCS8RSAKey(t, dsKey) {
		t.Fatal("DS key is not PKCS8 RSA")
	}
	if intermediateDER, err := os.ReadFile(intermediate); err != nil || string(intermediateDER) != string(parentDER) {
		t.Fatal("DS intermediate certificate differs from Dolphin client certificate")
	}
}

func validPKCS8RSAKey(t *testing.T, path string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return false
	}
	_, ok := key.(*rsa.PrivateKey)
	return ok
}

func TestIncompleteHTTPSPairIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "privkey.pem")
	if err := os.WriteFile(keyPath, []byte("existing secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := generateNormalCertificate(filepath.Join(dir, "fullchain.pem"), keyPath); err == nil {
		t.Fatal("incomplete pair was regenerated")
	}
	if data, err := os.ReadFile(keyPath); err != nil || string(data) != "existing secret" {
		t.Fatal("existing key changed")
	}
}

func TestMissingTLSFilesDoNotStopHTTP(t *testing.T) {
	dir := t.TempDir()
	// An incomplete normal pair and missing legacy files used to panic in a
	// background goroutine, terminating the whole server and its web panel.
	keyPath := filepath.Join(dir, "privkey.pem")
	if err := os.WriteFile(keyPath, []byte("existing key"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"naswii-chain.der", "nas-cert.der"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("incomplete certificate"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	on := true
	config := common.Config{
		CertPath: filepath.Join(dir, "fullchain.pem"), KeyPath: keyPath,
		CertPathWii: filepath.Join(dir, "naswii-chain.der"), KeyPathWii: filepath.Join(dir, "naswii-signed-key.pem"),
		CertPathDS: filepath.Join(dir, "nas-cert.der"), KeyPathDS: filepath.Join(dir, "nas-key.pem"),
		WiiCertPathDS:         filepath.Join(dir, "nwc.der"),
		EnableHTTPSExploitWii: &on, EnableHTTPSExploitDS: &on,
	}
	if setupTLS(config) {
		t.Fatal("HTTPS reported ready with no usable certificates")
	}
}

func TestDolphinSigningCertificateWhenPresent(t *testing.T) {
	parent, key, err := dolphinSigningCertificate()
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "naswii-chain.der")
	if err := generateExploitCertificate("naswii.nintendowifi.net", parent, key,
		certPath, filepath.Join(dir, "naswii-signed-key.pem"), "", false); err != nil {
		t.Fatal(err)
	}
	chain, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if certs, err := x509.ParseCertificates(chain); err != nil || len(certs) != 2 {
		t.Fatalf("Dolphin-signed chain: %v, count %d", err, len(certs))
	}
}

func TestPrepareAllHTTPSFiles(t *testing.T) {
	dir := t.TempDir()
	dolphin := t.TempDir()
	parentKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	parentTemplate := certificateTemplate("Wii client")
	parentDER, err := x509.CreateCertificate(rand.Reader, &parentTemplate, &parentTemplate, &parentKey.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dolphin, "clientca.pem"), parentDER, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dolphin, "clientcakey.pem"), x509.MarshalPKCS1PrivateKey(parentKey), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DWC_DOLPHIN_WII_DIR", dolphin)
	on := true
	config := common.Config{
		CertPath: filepath.Join(dir, "fullchain.pem"), KeyPath: filepath.Join(dir, "privkey.pem"),
		CertPathWii: filepath.Join(dir, "naswii-chain.der"), KeyPathWii: filepath.Join(dir, "naswii-signed-key.pem"),
		CertPathDS: filepath.Join(dir, "nas-cert.der"), KeyPathDS: filepath.Join(dir, "nas-key.pem"),
		WiiCertPathDS:         filepath.Join(dir, "nwc.der"),
		EnableHTTPSExploitWii: &on, EnableHTTPSExploitDS: &on,
	}
	if !setupTLS(config) {
		t.Fatal("generated HTTPS files were not loaded")
	}
	for _, path := range []string{config.CertPath, config.KeyPath, config.CertPathWii, config.KeyPathWii,
		config.CertPathDS, config.KeyPathDS, config.WiiCertPathDS} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("missing generated file %q: %v", path, err)
		}
	}
}
