package nas

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type certificateFile struct {
	path string
	data []byte
	mode os.FileMode
}

// GenerateHTTPSCertificates creates only complete, absent certificate sets.
// Existing files are never replaced, because a new key would invalidate its
// paired certificate and could break connected clients.
func GenerateHTTPSCertificates(config common.Config) error {
	wii := config.EnableHTTPSExploitWii != nil && *config.EnableHTTPSExploitWii
	ds := config.EnableHTTPSExploitDS != nil && *config.EnableHTTPSExploitDS
	var problems []error
	if err := generateNormalCertificate(config.CertPath, config.KeyPath); err != nil {
		logging.Error("NAS-TLS", "Cannot generate normal HTTPS certificate:", err)
		problems = append(problems, err)
	}
	if !wii && !ds {
		return errors.Join(problems...)
	}
	needWii := false
	if wii {
		var err error
		needWii, err = certificateSetMissing(config.CertPathWii, config.KeyPathWii)
		if err != nil {
			logging.Error("NAS-TLS", "Cannot generate Wii certificate:", err)
			problems = append(problems, err)
		}
	}
	needDS := false
	if ds {
		var err error
		needDS, err = certificateSetMissing(config.CertPathDS, config.KeyPathDS, config.WiiCertPathDS)
		if err != nil {
			logging.Error("NAS-TLS", "Cannot generate DS certificate:", err)
			problems = append(problems, err)
		}
	}
	if !(wii && needWii) && !(ds && needDS) {
		return errors.Join(problems...)
	}
	parent, parentKey, err := dolphinSigningCertificate()
	if err != nil {
		logging.Error("NAS-TLS", "Cannot generate Wii/DS certificates:", err)
		return errors.Join(append(problems, err)...)
	}
	if wii && needWii {
		if err := generateExploitCertificate("naswii.nintendowifi.net", parent, parentKey,
			config.CertPathWii, config.KeyPathWii, config.WiiCertPathDS, false); err != nil {
			logging.Error("NAS-TLS", "Cannot generate Wii certificate:", err)
			problems = append(problems, err)
		} else {
			logging.Notice("NAS-TLS", "Generated Wii HTTPS certificate and key")
		}
	}
	if ds && needDS {
		if err := generateExploitCertificate("nas.nintendowifi.net", parent, parentKey,
			config.CertPathDS, config.KeyPathDS, config.WiiCertPathDS, true); err != nil {
			logging.Error("NAS-TLS", "Cannot generate DS certificate:", err)
			problems = append(problems, err)
		} else {
			logging.Notice("NAS-TLS", "Generated DS HTTPS certificate and key")
		}
	}
	return errors.Join(problems...)
}

func certificateSetMissing(paths ...string) (bool, error) {
	missing := 0
	for _, path := range paths {
		if path == "" {
			return false, errors.New("a certificate path is empty")
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			missing++
		} else if err != nil {
			return false, fmt.Errorf("check %q: %w", path, err)
		}
	}
	if missing != 0 && missing != len(paths) {
		return false, errors.New("some certificate files already exist; keep the matching set or remove the incomplete set before regenerating")
	}
	return missing == len(paths), nil
}

func generateNormalCertificate(certPath, keyPath string) error {
	missing, err := certificateSetMissing(certPath, keyPath)
	if err != nil || !missing {
		return err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	template := certificateTemplate("naswii.nintendowifi.net")
	template.DNSNames = []string{"naswii.nintendowifi.net", "nas.nintendowifi.net", "localhost", "nas.localhost", "naswii.localhost"}
	template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	template.SignatureAlgorithm = x509.SHA256WithRSA
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := writeCertificateSet(
		certificateFile{certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644},
		certificateFile{keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600},
	); err != nil {
		return err
	}
	logging.Notice("NAS-TLS", "Generated self-signed HTTPS certificate and key (local use; browsers will not trust it automatically)")
	return nil
}

func certificateTemplate(host string) x509.Certificate {
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	return x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{Country: []string{"US"}, Organization: []string{"Nintendo of America Inc."},
			OrganizationalUnit: []string{"Nintendo Wifi Network"}, CommonName: host},
		NotBefore:   time.Now().Add(-24 * time.Hour),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{host},
	}
}

func dolphinSigningCertificate() (*x509.Certificate, *rsa.PrivateKey, error) {
	for _, dir := range dolphinCertificateDirs() {
		certPath, keyPath := filepath.Join(dir, "clientca.pem"), filepath.Join(dir, "clientcakey.pem")
		cert, certErr := os.ReadFile(certPath)
		key, keyErr := os.ReadFile(keyPath)
		if certErr != nil || keyErr != nil {
			continue
		}
		if block, _ := pem.Decode(cert); block != nil {
			cert = block.Bytes
		}
		parent, err := x509.ParseCertificate(cert)
		if err != nil {
			return nil, nil, fmt.Errorf("Dolphin certificate in %q: %w", dir, err)
		}
		if block, _ := pem.Decode(key); block != nil {
			key = block.Bytes
		}
		rsaKey, err := x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			parsed, parseErr := x509.ParsePKCS8PrivateKey(key)
			if parseErr != nil {
				return nil, nil, fmt.Errorf("Dolphin client key in %q: %w", dir, err)
			}
			var ok bool
			rsaKey, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				return nil, nil, errors.New("Dolphin client key is not RSA")
			}
		}
		public, ok := parent.PublicKey.(*rsa.PublicKey)
		if !ok || public.N.Cmp(rsaKey.PublicKey.N) != 0 || public.E != rsaKey.PublicKey.E {
			return nil, nil, fmt.Errorf("Dolphin certificate and key in %q do not match", dir)
		}
		return parent, rsaKey, nil
	}
	return nil, nil, errors.New("Dolphin clientca.pem and clientcakey.pem not found; set DWC_DOLPHIN_WII_DIR to their folder")
}

func dolphinCertificateDirs() []string {
	dirs := []string{}
	if dir := os.Getenv("DWC_DOLPHIN_WII_DIR"); dir != "" {
		dirs = append(dirs, dir)
	}
	dirs = append(dirs, ".")
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("APPDATA"); dir != "" {
			dirs = append(dirs, filepath.Join(dir, "Dolphin Emulator", "Wii"))
		}
	} else if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "dolphin-emu", "Wii"),
			filepath.Join(home, "Library", "Application Support", "Dolphin", "Wii"))
	}
	return dirs
}

func generateExploitCertificate(host string, parent *x509.Certificate, parentKey *rsa.PrivateKey,
	certPath, keyPath, intermediatePath string, ds bool) error {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return err
	}
	template := certificateTemplate(host)
	template.SignatureAlgorithm = x509.SHA1WithRSA // Required by legacy Wii/DS clients.
	der, err := x509.CreateCertificate(rand.Reader, &template, parent, &key.PublicKey, parentKey)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	files := []certificateFile{{keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600}}
	if ds {
		files = append(files, certificateFile{certPath, der, 0o644}, certificateFile{intermediatePath, parent.Raw, 0o644})
	} else {
		chain := append(append([]byte{}, der...), parent.Raw...)
		files = append(files, certificateFile{certPath, chain, 0o644})
	}
	return writeCertificateSet(files...)
}

func writeCertificateSet(files ...certificateFile) error {
	created := []string{}
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o700); err != nil {
			removeCreatedCertificates(created)
			return err
		}
		out, err := os.OpenFile(file.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.mode)
		if err != nil {
			removeCreatedCertificates(created)
			return fmt.Errorf("create %q: %w", file.path, err)
		}
		created = append(created, file.path)
		_, writeErr := out.Write(file.data)
		closeErr := out.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			removeCreatedCertificates(created)
			return fmt.Errorf("write %q: %w", file.path, err)
		}
	}
	return nil
}

func removeCreatedCertificates(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
