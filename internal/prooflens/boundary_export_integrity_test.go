package prooflens

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

const (
	testOfflineLock = ".write.lock"
	testOfflineJSON = "out.json"
	testOfflineHTML = "out.html"
)

func testOfflineOutputPaths(root string) (string, string, string) {
	return filepath.Join(root, testOfflineJSON), filepath.Join(root, testOfflineHTML), filepath.Join(root, testOfflineLock)
}

func testOfflineNoFiles(t *testing.T, root string) {
	t.Helper()
	for _, p := range []string{testOfflineJSON, testOfflineHTML, testOfflineLock} {
		if _, err := os.Lstat(filepath.Join(root, p)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected artifact %s: %v", p, err)
		}
	}
}

func TestSharedOfflineWriterWritesVerifiedBytesOnce(t *testing.T) {
	root := t.TempDir()
	jsonData := []byte("{\"a\":1}\n")
	htmlData := []byte("<!doctype html><p>INCONCLUSIVE</p>")
	if err := saveOfflineProofPairOnce(root, testOfflineLock, testOfflineJSON, testOfflineHTML, jsonData, htmlData); err != nil {
		t.Fatal(err)
	}
	j, h, lock := testOfflineOutputPaths(root)
	for _, item := range []struct {
		path string
		data []byte
	}{{j, jsonData}, {h, htmlData}} {
		b, err := os.ReadFile(item.path)
		if err != nil || !bytes.Equal(b, item.data) {
			t.Fatalf("mismatched output %s: %v", item.path, err)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(item.path)
			if err != nil || info.Mode().Perm()&0077 != 0 {
				t.Fatalf("output permissions too broad: %s (%v)", item.path, err)
			}
		}
	}
	if _, err := os.Lstat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("lock remains after success")
	}
	if err := saveOfflineProofPairOnce(root, testOfflineLock, testOfflineJSON, testOfflineHTML, jsonData, htmlData); err == nil {
		t.Fatal("existing outputs overwritten")
	}
	b, _ := os.ReadFile(j)
	if !bytes.Equal(b, jsonData) {
		t.Fatal("failed retry changed output")
	}
}

func TestSharedOfflineWriterRefusesInvalidSizes(t *testing.T) {
	for _, tc := range []struct {
		name string
		json []byte
		html []byte
	}{
		{"empty JSON", nil, []byte("html")},
		{"empty HTML", []byte("{}"), nil},
		{"oversized JSON", bytes.Repeat([]byte("a"), reporting.MaxJSONBytes+1), []byte("html")},
		{"oversized HTML", []byte("{}"), bytes.Repeat([]byte("b"), reporting.MaxHTMLBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := saveOfflineProofPairOnce(root, testOfflineLock, testOfflineJSON, testOfflineHTML, tc.json, tc.html); err == nil {
				t.Fatal("invalid output accepted")
			}
			testOfflineNoFiles(t, root)
		})
	}
}

func TestSharedOfflineWriterNeverModifiesExistingFiles(t *testing.T) {
	for _, name := range []string{testOfflineLock, testOfflineJSON, testOfflineHTML} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, name)
			if err := os.WriteFile(p, []byte("do not alter"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := saveOfflineProofPairOnce(root, testOfflineLock, testOfflineJSON, testOfflineHTML, []byte("{}"), []byte("html")); err == nil {
				t.Fatal("existing file accepted")
			}
			b, err := os.ReadFile(p)
			if err != nil || string(b) != "do not alter" {
				t.Fatalf("existing file altered: %v", err)
			}
		})
	}
}

func TestSharedOfflineWriterRollsBackOnInterruption(t *testing.T) {
	root := t.TempDir()
	err := saveOfflineProofPairWithHook(root, testOfflineLock, testOfflineJSON, testOfflineHTML,
		[]byte("{}"), []byte("html"), func(index int, path string) error {
			if index == 0 {
				return errors.New("simulated power interruption")
			}
			return nil
		})
	if err == nil {
		t.Fatal("interrupted export accepted")
	}
	testOfflineNoFiles(t, root)
}

func TestSharedOfflineWriterDetectsTamperedOutput(t *testing.T) {
	root := t.TempDir()
	err := saveOfflineProofPairWithHook(root, testOfflineLock, testOfflineJSON, testOfflineHTML,
		[]byte("0123456789"), []byte("html"), func(index int, path string) error {
			if index == 0 {
				return os.WriteFile(path, []byte("9876543210"), 0600)
			}
			return nil
		})
	if err == nil {
		t.Fatal("modified output accepted")
	}
	testOfflineNoFiles(t, root)
}

func TestSharedOfflineWriterPreservesConcurrentExistingDestination(t *testing.T) {
	root := t.TempDir()
	j, h, lock := testOfflineOutputPaths(root)
	err := saveOfflineProofPairWithHook(root, testOfflineLock, testOfflineJSON, testOfflineHTML,
		[]byte("{}"), []byte("html"), func(index int, path string) error {
			if index == 0 {
				return os.WriteFile(h, []byte("someone else's content"), 0600)
			}
			return nil
		})
	if err == nil {
		t.Fatal("raced destination accepted")
	}
	if _, err := os.Lstat(j); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("own JSON was not rolled back")
	}
	b, err := os.ReadFile(h)
	if err != nil || string(b) != "someone else's content" {
		t.Fatal("exporter damaged concurrent file")
	}
	if _, err := os.Lstat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned lock remains")
	}
}

func TestSharedOfflineWriterDoesNotRemoveReplacedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows may refuse to unlink a file with a held write handle")
	}
	root := t.TempDir()
	j, h, lock := testOfflineOutputPaths(root)
	err := saveOfflineProofPairWithHook(root, testOfflineLock, testOfflineJSON, testOfflineHTML,
		[]byte("{}"), []byte("html"), func(index int, path string) error {
			if index == 0 {
				if err := os.Remove(path); err != nil {
					return err
				}
				return os.WriteFile(path, []byte("not our file"), 0600)
			}
			return nil
		})
	if err == nil {
		t.Fatal("replaced file accepted")
	}
	b, err := os.ReadFile(j)
	if err != nil || string(b) != "not our file" {
		t.Fatal("exporter deleted a replaced path")
	}
	if _, err := os.Lstat(h); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unexpected HTML file")
	}
	if _, err := os.Lstat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned lock remains")
	}
}
