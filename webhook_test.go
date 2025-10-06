package bote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCertificate_Errors(t *testing.T) {
	dir := t.TempDir()
	badCert := filepath.Join(dir, "cert.pem")
	badKey := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(badCert, []byte("bad"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badKey, []byte("bad"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateCertificate(badCert, badKey, noopLogger{}); err == nil {
		t.Fatalf("expected error on invalid cert pair")
	}
}
