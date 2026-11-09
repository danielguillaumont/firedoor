package prooflens

// Shared v0.8 offline writer for the two fixed, Git-ignored JSON/HTML exports.
// It verifies bytes and filesystem identity before reporting success. These
// checks establish local write consistency, NOT source authentication, causal
// enforcement or a crash-atomic multi-file transaction.
import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/danielguillaumont/firedoor/internal/reporting"
)

var errOfflinePairWrite = errors.New("offline proof pair write refused")

type writtenExport struct {
	path string
	info os.FileInfo
}

// removeOwnedExport never unlinks a path that has been replaced since this
// process created it. A changed path is left in place and the caller fails.
func removeOwnedExport(owned writtenExport) {
	if current, err := os.Lstat(owned.path); err == nil && os.SameFile(owned.info, current) {
		_ = os.Remove(owned.path)
	}
}

// saveOfflineProofPairOnce writes only fixed paths selected by the callers.
// Both paths must already have passed the root, ignore and tracking checks.
func saveOfflineProofPairOnce(base, lockRel, jsonRel, htmlRel string, jsonData, htmlData []byte) error {
	return saveOfflineProofPairWithHook(base, lockRel, jsonRel, htmlRel, jsonData, htmlData, nil)
}

// The hook is nil in production. Tests use it to simulate interruptions or
// modifications after a file is written, before integrity verification.
func saveOfflineProofPairWithHook(base, lockRel, jsonRel, htmlRel string, jsonData, htmlData []byte, afterWrite func(index int, path string) error) error {
	if len(jsonData) == 0 || len(jsonData) > reporting.MaxJSONBytes ||
		len(htmlData) == 0 || len(htmlData) > reporting.MaxHTMLBytes {
		return errOfflinePairWrite
	}
	lockPath := filepath.Join(base, filepath.FromSlash(lockRel))
	lockFile, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errOfflinePairWrite
	}
	lockInfo, statErr := lockFile.Stat()
	closeErr := lockFile.Close()
	if statErr != nil || closeErr != nil || lockInfo == nil {
		if lockInfo != nil {
			removeOwnedExport(writtenExport{path: lockPath, info: lockInfo})
		}
		return errOfflinePairWrite
	}
	defer removeOwnedExport(writtenExport{path: lockPath, info: lockInfo})

	files := []struct {
		path string
		data []byte
	}{
		{filepath.Join(base, filepath.FromSlash(jsonRel)), jsonData},
		{filepath.Join(base, filepath.FromSlash(htmlRel)), htmlData},
	}
	for _, output := range files {
		if _, err := os.Lstat(output.path); !errors.Is(err, os.ErrNotExist) {
			return errOfflinePairWrite
		}
	}

	created := make([]writtenExport, 0, 2)
	openFiles := make([]*os.File, 0, 2)
	committed := false
	defer func() {
		for _, f := range openFiles {
			if f != nil {
				_ = f.Close()
			}
		}
		if !committed {
			for _, file := range created {
				removeOwnedExport(file)
			}
		}
	}()

	for i, output := range files {
		f, err := os.OpenFile(output.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return errOfflinePairWrite
		}
		info, statErr := f.Stat()
		openFiles = append(openFiles, f)
		if statErr != nil || info == nil || !info.Mode().IsRegular() {
			return errOfflinePairWrite
		}
		created = append(created, writtenExport{path: output.path, info: info})
		n, writeErr := f.Write(output.data)
		if writeErr == nil && n != len(output.data) {
			writeErr = io.ErrShortWrite
		}
		if writeErr == nil {
			writeErr = f.Sync()
		}
		if writeErr != nil {
			return errOfflinePairWrite
		}
		if afterWrite != nil {
			if err := afterWrite(i, output.path); err != nil {
				return errOfflinePairWrite
			}
		}
		if err := verifyOwnedExport(created[i], output.data); err != nil {
			return errOfflinePairWrite
		}
	}
	for i, file := range created {
		if err := verifyOwnedExport(file, files[i].data); err != nil {
			return errOfflinePairWrite
		}
	}
	if current, err := os.Lstat(lockPath); err != nil || !os.SameFile(lockInfo, current) {
		return errOfflinePairWrite
	}
	for i, f := range openFiles {
		if err := f.Close(); err != nil {
			return errOfflinePairWrite
		}
		openFiles[i] = nil
	}
	committed = true
	return nil
}

// verifyOwnedExport reopens the exact inode/file ID just created and checks
// its bounded bytes match the renderer's output. It does not authenticate
// historical evidence, and cannot prevent later edits by a local user.
func verifyOwnedExport(file writtenExport, want []byte) error {
	current, err := os.Lstat(file.path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(file.info, current) || current.Size() != int64(len(want)) {
		return errOfflinePairWrite
	}
	opened, err := os.Open(file.path)
	if err != nil {
		return errOfflinePairWrite
	}
	defer opened.Close()
	openedInfo, err := opened.Stat()
	if err != nil || !os.SameFile(file.info, openedInfo) || openedInfo.Size() != int64(len(want)) {
		return errOfflinePairWrite
	}
	got, err := io.ReadAll(io.LimitReader(opened, int64(len(want))+1))
	if err != nil || !bytes.Equal(got, want) {
		return errOfflinePairWrite
	}
	return nil
}
