package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"xrayctl/internal"
)

func TestValidateTarForRootExtractionAcceptsNormalBackupPaths(t *testing.T) {
	archive := writeTarGz(t, []testTarEntry{
		{name: "etc/xrayctl/config.yaml", body: "uuid: abc\n"},
		{name: "usr/local/etc/xray/cert/xray.crt", body: "cert"},
	})

	if err := validateTarForRootExtraction(readArchiveBytes(t, archive), safeRestorePrefixes, nil); err != nil {
		t.Fatalf("validateTarForRootExtraction returned error: %v", err)
	}
}

func TestValidateTarForRootExtractionAcceptsDirectoryEntries(t *testing.T) {
	// GNU tar emits directory entries (e.g. "etc/xrayctl/") alongside
	// the files they contain. path.Clean strips the trailing slash, so
	// the prefix check must match the directory name itself.
	archive := writeTarGz(t, []testTarEntry{
		{name: "etc/xrayctl/", dir: true},
		{name: "etc/xrayctl/config.yaml", body: "uuid: abc\n"},
	})

	if err := validateTarForRootExtraction(readArchiveBytes(t, archive), safeRestorePrefixes, nil); err != nil {
		t.Fatalf("validateTarForRootExtraction returned error for tar with dir entries: %v", err)
	}
}

func TestValidateTarForRootExtractionRejectsUnsafePaths(t *testing.T) {
	cases := []struct {
		name  string
		entry testTarEntry
		want  string
	}{
		{
			name:  "absolute path",
			entry: testTarEntry{name: "/etc/passwd", body: "x"},
			want:  "absolute path",
		},
		{
			name:  "parent traversal",
			entry: testTarEntry{name: "etc/../passwd", body: "x"},
			want:  "parent traversal",
		},
		{
			name:  "absolute symlink target",
			entry: testTarEntry{name: "etc/xrayctl/link", linkname: "/etc/shadow"},
			want:  "absolute target",
		},
		{
			name:  "path outside permitted prefixes",
			entry: testTarEntry{name: "etc/passwd", body: "x"},
			want:  "not in permitted restore paths",
		},
		{
			name:  "traversing symlink target",
			entry: testTarEntry{name: "etc/xrayctl/link", linkname: "../../shadow"},
			want:  "parent traversal target",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			archive := writeTarGz(t, []testTarEntry{tc.entry})

			err := validateTarForRootExtraction(readArchiveBytes(t, archive), safeRestorePrefixes, nil)
			if err == nil {
				t.Fatal("validateTarForRootExtraction returned nil, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateTarForRootExtractionRejectsPathsThroughArchiveSymlink(t *testing.T) {
	archive := writeTarGz(t, []testTarEntry{
		{name: "etc/xrayctl/link", linkname: "safe-target"},
		{name: "etc/xrayctl/link/config.yaml", body: "uuid: abc\n"},
	})

	err := validateTarForRootExtraction(readArchiveBytes(t, archive), safeRestorePrefixes, nil)
	if err == nil {
		t.Fatal("validateTarForRootExtraction returned nil, want archive symlink traversal error")
	}
	if !strings.Contains(err.Error(), "traverses archive symlink") {
		t.Fatalf("error = %q, want archive symlink traversal context", err)
	}
}

func TestValidateTarRejectsHardlinkToUnpermittedTarget(t *testing.T) {
	archive := writeHardlinkTarGz(t, "etc/xrayctl/link", "etc/passwd")
	err := validateTarForRootExtraction(readArchiveBytes(t, archive), safeRestorePrefixes, nil)
	if err == nil {
		t.Fatal("validateTarForRootExtraction returned nil, want hardlink target prefix error")
	}
	if !strings.Contains(err.Error(), "not in permitted restore paths") {
		t.Fatalf("error = %q, want hardlink target prefix error", err)
	}
}

func TestDirToArchivePrefix(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/etc/xrayctl/", "etc/xrayctl/"},
		{"/etc/xrayctl", "etc/xrayctl/"},
		{"/data/certs", "data/certs/"},
		{"relative/path", ""},
		{"/", ""},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := dirToArchivePrefix(tc.path)
			if got != tc.want {
				t.Fatalf("dirToArchivePrefix(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestFileToArchivePath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/usr/local/etc/xray/config.json", "usr/local/etc/xray/config.json"},
		{"/data/certs/xray.crt", "data/certs/xray.crt"},
		{"/etc/xray.json", "etc/xray.json"},
		{"relative/path", ""},
		{"/", ""},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := fileToArchivePath(tc.path)
			if got != tc.want {
				t.Fatalf("fileToArchivePath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestValidateTarForRootExtractionAcceptsExactFilePaths(t *testing.T) {
	archive := writeTarGz(t, []testTarEntry{
		{name: "etc/xray.json", body: `{"log": {}}`},
	})

	err := validateTarForRootExtraction(readArchiveBytes(t, archive), nil, []string{"etc/xray.json"})
	if err != nil {
		t.Fatalf("expected no error for exact allowed path, got: %v", err)
	}
}

func TestValidateTarForRootExtractionRejectsParentDirWhenOnlyExactFileAllowed(t *testing.T) {
	archive := writeTarGz(t, []testTarEntry{
		{name: "etc/xray.json", body: `{"log": {}}`},
		{name: "etc/passwd", body: "root:x:0:0:"},
	})

	err := validateTarForRootExtraction(readArchiveBytes(t, archive), nil, []string{"etc/xray.json"})
	if err == nil {
		t.Fatal("expected error for etc/passwd outside exact paths")
	}
	if !strings.Contains(err.Error(), "not in permitted restore paths") {
		t.Fatalf("error = %q, want path rejection for etc/passwd", err)
	}
}

func TestRestoreRejectsUnsafeArchiveBeforeCommands(t *testing.T) {
	archive := writeTarGz(t, []testTarEntry{{name: "../etc/passwd", body: "x"}})
	recorder := &restoreCommandRecorder{}
	stubDefaultRunner(t, recorder)
	stubRestoreExtractor(t, func([]byte) error {
		t.Fatal("Restore extracted archive after validation failed")
		return nil
	})

	err := Restore(archive)
	if err == nil {
		t.Fatal("Restore returned nil, want unsafe archive error")
	}
	if len(recorder.calls) != 0 {
		t.Fatalf("Restore ran commands before validation failed: %v", recorder.calls)
	}
}

func TestRestoreReturnsRestartErrorAfterSuccessfulExtract(t *testing.T) {
	restartErr := errors.New("systemctl start failed")
	archive := writeTarGz(t, []testTarEntry{{name: "etc/xrayctl/config.yaml", body: "uuid: abc\n"}})
	wantArchive := readArchiveBytes(t, archive)
	extracted := false
	recorder := &restoreCommandRecorder{
		errByCall: map[string]error{
			"systemctl start xray nginx warp-svc": restartErr,
		},
	}
	stubDefaultRunner(t, recorder)
	stubRestoreExtractor(t, func(got []byte) error {
		extracted = true
		if !bytes.Equal(got, wantArchive) {
			t.Fatal("Restore extracted bytes different from the archive bytes read before validation")
		}

		return nil
	})

	err := Restore(archive)
	if err == nil {
		t.Fatal("Restore returned nil, want restart error")
	}
	if !strings.Contains(err.Error(), "restart services after restore") {
		t.Fatalf("Restore error = %q, want restart context", err)
	}
	if !extracted {
		t.Fatal("Restore did not extract archive before restart failure")
	}
}

type testTarEntry struct {
	name     string
	body     string
	linkname string
	dir      bool
	hardlink bool
}

func writeHardlinkTarGz(t *testing.T, name, linkname string) string {
	t.Helper()
	path := t.TempDir() + "/hardlink.tar.gz"
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     name,
		Mode:     0o600,
		Typeflag: tar.TypeLink,
		Linkname: linkname,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write header: %v", err)
	}
	tw.Close()
	gz.Close()
	file.Close()
	return path
}

func readArchiveBytes(t *testing.T, path string) []byte {
	t.Helper()

	archiveBytes, err := os.ReadFile(path) //nolint:gosec // test archive path is in t.TempDir
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}

	return archiveBytes
}

func writeTarGz(t *testing.T, entries []testTarEntry) string {
	t.Helper()

	path := t.TempDir() + "/backup.tar.gz"
	file, err := os.Create(path) //nolint:gosec // test archive path is in t.TempDir
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)

	for _, entry := range entries {
		header := &tar.Header{
			Name: entry.name,
			Mode: 0o600,
		}

		if entry.linkname != "" {
			header.Typeflag = tar.TypeSymlink
			header.Linkname = entry.linkname
		} else if entry.dir {
			header.Typeflag = tar.TypeDir
			header.Size = 0
		} else {
			header.Typeflag = tar.TypeReg
			header.Size = int64(len(entry.body))
		}

		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if entry.body != "" {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatalf("write tar body: %v", err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	return path
}

type restoreCommandRecorder struct {
	calls     []string
	errByCall map[string]error
}

func (r *restoreCommandRecorder) Run(_ context.Context, name string, args ...string) (string, error) {
	call := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, call)
	if err := r.errByCall[call]; err != nil {
		return "", err
	}

	return "", nil
}

func (r *restoreCommandRecorder) RunSilent(ctx context.Context, name string, args ...string) (string, error) {
	return r.Run(ctx, name, args...)
}

func (r *restoreCommandRecorder) RunWithSudo(ctx context.Context, name string, args ...string) (string, error) {
	return r.Run(ctx, name, args...)
}

func stubDefaultRunner(t *testing.T, runner internal.CommandRunner) {
	t.Helper()

	origRunner := internal.DefaultRunner
	internal.DefaultRunner = runner

	t.Cleanup(func() {
		internal.DefaultRunner = origRunner
	})
}

func stubRestoreExtractor(t *testing.T, extractor func([]byte) error) {
	t.Helper()

	origExtractor := restoreArchiveExtractor
	restoreArchiveExtractor = extractor

	t.Cleanup(func() {
		restoreArchiveExtractor = origExtractor
	})
}
