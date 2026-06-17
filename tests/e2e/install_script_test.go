package e2e_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestInstallScript exercises install.sh end-to-end against a local server that
// stands in for GitHub releases. It is a regression guard for issue #34, where a
// 404 on the checksum sidecar caused curl (without --fail) to write the body
// "Not Found" into the checksum file, which the script then parsed as the
// expected checksum — producing the confusing "Expected: Not" failure and, worse,
// could install a non-binary error page when checksum tooling was absent.
func TestInstallScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is a POSIX shell script; not run on Windows")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	scriptPath := repoFile(t, "install.sh")

	// Map the script's GOOS->os and GOARCH->arch naming.
	osName := runtime.GOOS // "linux" or "darwin" — matches detect_os output
	archName := runtime.GOARCH
	switch archName {
	case "amd64", "arm64":
		// already matches detect_arch output
	default:
		t.Skipf("unsupported test arch %q", archName)
	}

	const version = "v9.9.9"
	binaryName := "mockd-" + osName + "-" + archName
	fakeBinary := []byte("#!/bin/sh\necho mockd version " + version + "\n")
	sum := sha256.Sum256(fakeBinary)
	checksumLine := hex.EncodeToString(sum[:]) + "  " + binaryName + "\n"

	t.Run("happy path installs and verifies checksum", func(t *testing.T) {
		srv := newReleaseServer(t, releaseConfig{
			version:      version,
			binaryName:   binaryName,
			binary:       fakeBinary,
			checksumBody: checksumLine,
			checksum404:  false,
		})
		defer srv.Close()

		installDir := t.TempDir()
		out, err := runInstall(t, scriptPath, srv.URL, version, installDir)
		if err != nil {
			t.Fatalf("install.sh failed unexpectedly: %v\noutput:\n%s", err, out)
		}
		if !strings.Contains(out, "Checksum verified") {
			t.Errorf("expected checksum to be verified, output:\n%s", out)
		}
		installed := filepath.Join(installDir, "mockd")
		if data, rerr := os.ReadFile(installed); rerr != nil {
			t.Fatalf("binary not installed: %v", rerr)
		} else if string(data) != string(fakeBinary) {
			t.Errorf("installed binary content mismatch")
		}
	})

	// Core regression: a 404 on the checksum URL must fail loudly and must NOT
	// surface as "Expected: Not" (the symptom from issue #34).
	t.Run("checksum 404 fails clearly without Expected: Not", func(t *testing.T) {
		srv := newReleaseServer(t, releaseConfig{
			version:     version,
			binaryName:  binaryName,
			binary:      fakeBinary,
			checksum404: true,
		})
		defer srv.Close()

		installDir := t.TempDir()
		out, err := runInstall(t, scriptPath, srv.URL, version, installDir)
		if err == nil {
			t.Fatalf("install.sh should have failed on checksum 404, output:\n%s", out)
		}
		if strings.Contains(out, "Expected: Not") {
			t.Errorf("regression: script parsed error body as checksum (Expected: Not)\noutput:\n%s", out)
		}
		if !strings.Contains(out, "Could not download checksum") {
			t.Errorf("expected a clear checksum-download error, output:\n%s", out)
		}
		if _, serr := os.Stat(filepath.Join(installDir, "mockd")); serr == nil {
			t.Errorf("binary should NOT have been installed when checksum is unavailable")
		}
	})

	// A 404 on the binary itself must also fail loudly rather than installing the
	// server's "Not Found" error page as an executable.
	t.Run("binary 404 fails without installing error page", func(t *testing.T) {
		srv := newReleaseServer(t, releaseConfig{
			version:    version,
			binaryName: binaryName,
			binary404:  true,
		})
		defer srv.Close()

		installDir := t.TempDir()
		out, err := runInstall(t, scriptPath, srv.URL, version, installDir)
		if err == nil {
			t.Fatalf("install.sh should have failed on binary 404, output:\n%s", out)
		}
		if !strings.Contains(out, "Could not download") {
			t.Errorf("expected a clear download error, output:\n%s", out)
		}
		if _, serr := os.Stat(filepath.Join(installDir, "mockd")); serr == nil {
			t.Errorf("error page should NOT have been installed as the binary")
		}
	})

	// A checksum file containing garbage (e.g. an unexpected HTML/error body that
	// still returns HTTP 200) must be rejected as not-a-digest.
	t.Run("malformed checksum body is rejected", func(t *testing.T) {
		srv := newReleaseServer(t, releaseConfig{
			version:      version,
			binaryName:   binaryName,
			binary:       fakeBinary,
			checksumBody: "Not Found\n",
		})
		defer srv.Close()

		installDir := t.TempDir()
		out, err := runInstall(t, scriptPath, srv.URL, version, installDir)
		if err == nil {
			t.Fatalf("install.sh should have failed on malformed checksum, output:\n%s", out)
		}
		if strings.Contains(out, "Expected: Not") {
			t.Errorf("regression: script parsed 'Not Found' as checksum\noutput:\n%s", out)
		}
		if !strings.Contains(out, "not a valid sha256 digest") {
			t.Errorf("expected a clear malformed-checksum error, output:\n%s", out)
		}
	})
}

type releaseConfig struct {
	version      string
	binaryName   string
	binary       []byte
	binary404    bool
	checksumBody string
	checksum404  bool
}

// newReleaseServer mimics the subset of GitHub's release-download API that
// install.sh uses: /<version>/<binaryName> and /<version>/<binaryName>.sha256.
func newReleaseServer(t *testing.T, cfg releaseConfig) *httptest.Server {
	t.Helper()
	binPath := "/" + cfg.version + "/" + cfg.binaryName
	sumPath := binPath + ".sha256"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case binPath:
			if cfg.binary404 {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			_, _ = w.Write(cfg.binary)
		case sumPath:
			if cfg.checksum404 {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(cfg.checksumBody))
		default:
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
}

func runInstall(t *testing.T, scriptPath, baseURL, version, installDir string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", scriptPath)
	cmd.Env = append(os.Environ(),
		"MOCKD_DOWNLOAD_BASE="+baseURL,
		"VERSION="+version,
		"INSTALL_DIR="+installDir,
		"MOCKD_NO_TELEMETRY=1",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// repoFile resolves a path relative to the repository root from within the
// tests/e2e package directory.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	wd, err := os.Getwd() // .../tests/e2e
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	p := filepath.Join(root, rel)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("could not find %s at %s: %v", rel, p, err)
	}
	return p
}
