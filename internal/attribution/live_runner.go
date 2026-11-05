package attribution

// Guarded live execution coordinator. A real Kubernetes driver is only wired
// by the separately consented command; unit tests inject a fake. The single
// process lock intentionally persists on uncertainty or interrupted cleanup.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const LiveRunSchema = "firedoor.v04.live-aba-run.v1"
const liveLockName = ".v04-live-aba.lock"

// FinalAudit independently verifies that the entire owned policy inventory
// has returned to its approved hardened state after the experiment.
type FinalAudit func(context.Context) error

type LiveRunResult struct {
	Schema          string          `json:"schema"`
	Cluster         string          `json:"cluster"`
	Context         string          `json:"context"`
	ExperimentID    string          `json:"experiment_id"`
	CapturedUTC     time.Time       `json:"captured_utc"`
	Status          string          `json:"status"`
	JournalPath     string          `json:"journal_path"`
	ReportPath      string          `json:"report_path"`
	LockPath        string          `json:"lock_path"`
	LockReleased    bool            `json:"lock_released"`
	RecoveryPending bool            `json:"recovery_pending"`
	Report          LifecycleReport `json:"report"`
	Journal         RecoverySummary `json:"journal"`
	Limitations     []string        `json:"limitations"`
}

func selectApprovedExperiment(id string) (Experiment, error) {
	for _, ex := range ApprovedPlan().Experiments {
		if id == ex.ID {
			return ex, approvedExperiment(ex)
		}
	}
	return Experiment{}, errors.New("experiment must be one of the two immutable Firedoor v0.4 IDs")
}

func inspectOldLiveJournals(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasPrefix(name, "v04-live-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		if !ent.Type().IsRegular() {
			return fmt.Errorf("unsafe previous recovery journal %s", name)
		}
		sum, err := InspectRecoveryJournal(filepath.Join(dir, name))
		if err != nil || sum.PendingManualReview {
			return fmt.Errorf("previous journal %s is unresolved or corrupted; stop for recovery review: %v", name, err)
		}
	}
	return nil
}

func evidenceFolder(repoRoot string) (string, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.HasPrefix(strings.ReplaceAll(string(gomod), "\r\n", "\n"), "module github.com/danielguillaumont/firedoor\n") {
		return "", errors.New("run from the root of the Firedoor repository")
	}
	parent := filepath.Join(root, "docs", "evidence")
	dir := filepath.Join(parent, "local")
	// Never follow an existing symlink even while creating a missing
	// evidence directory. The Go repository root itself is operator-chosen.
	for _, p := range []string{filepath.Join(root, "docs"), parent, dir} {
		info, err := os.Lstat(p)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return "", fmt.Errorf("recovery directory is not an ordinary directory: %s", p)
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	for _, p := range []string{filepath.Join(root, "docs"), parent, dir} {
		info, err := os.Lstat(p)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("recovery directory identity changed: %s", p)
		}
	}
	return dir, nil
}

func newLiveFileName(ex Experiment) (string, error) {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("cannot generate unique evidence ID: %w", err)
	}
	return fmt.Sprintf("v04-live-%s-%s-%s", ex.RouteID, time.Now().UTC().Format("20060102T150405Z"), hex.EncodeToString(nonce[:])), nil
}

func writeExclusiveJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// RunOneLiveWithRecovery handles only ONE approved experiment. The supplied
// driver may mutate Kubernetes, so never construct it without explicit human
// consent and a fresh read-only live preflight. It creates an exclusive lock
// before checking old journals, and leaves the lock behind on every failure.
// This includes failures after cleanup, to force human review before retry.
func RunOneLiveWithRecovery(ctx context.Context, repoRoot, experimentID string, driver LifecycleDriver, finalAudit FinalAudit) (result LiveRunResult, runErr error) {
	result = LiveRunResult{Schema: LiveRunSchema, Cluster: OwnedCluster, Context: OwnedContext,
		ExperimentID: experimentID, CapturedUTC: time.Now().UTC(), Status: "rejected",
		RecoveryPending: true, Limitations: []string{
			"Only the specified owned kind lab is supported; never run against other clusters.",
			"A/B/A HTTP observations support bounded policy-effect evidence, not packet-level proof of causality.",
			"A crash, power failure or SIGKILL can bypass in-process cleanup; a retained lock requires manual review.",
			"Recovery journals are local and unauthenticated; inspect the live cluster independently.",
		}}
	ex, err := selectApprovedExperiment(experimentID)
	if err != nil {
		return result, err
	}
	if driver == nil || finalAudit == nil {
		return result, errors.New("guarded driver and independent final audit required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	dir, err := evidenceFolder(repoRoot)
	if err != nil {
		return result, err
	}
	result.LockPath = filepath.Join(dir, liveLockName)
	lock, err := os.OpenFile(result.LockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, fmt.Errorf("another run or unresolved recovery lock exists; do not delete it without review: %w", err)
	}
	// Incomplete lock writes still leave an exclusive recovery marker.
	if _, err := lock.WriteString("Firedoor v0.4 live policy experiment lock\n" + ex.ID + "\n" + time.Now().UTC().Format(time.RFC3339Nano) + "\n"); err != nil {
		lock.Close()
		return result, err
	}
	if err := lock.Sync(); err != nil {
		lock.Close()
		return result, err
	}
	if err := lock.Close(); err != nil {
		return result, err
	}
	if err := inspectOldLiveJournals(dir); err != nil {
		return result, err // retained lock intentionally prevents bypass
	}
	stem, err := newLiveFileName(ex)
	if err != nil {
		return result, err
	}
	result.JournalPath = filepath.Join(dir, stem+".jsonl")
	result.ReportPath = filepath.Join(dir, stem+".json")
	journal, err := NewRecoveryJournal(result.JournalPath)
	if err != nil {
		return result, err
	}
	wrapped, err := NewJournaledLifecycleDriver(driver, journal)
	if err != nil {
		return result, err
	}
	result.Status = "running"
	result.Report, runErr = (Lifecycle{Driver: wrapped}).RunOne(ctx, ex)
	result.Status = result.Report.Status
	result.Journal, err = InspectRecoveryJournal(result.JournalPath)
	if err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("post-run recovery journal unreadable: %w", err))
	}
	if runErr == nil && (result.Status != "aba_matched" || !result.Report.CleanupVerified || !result.Report.RestorationVerified ||
		result.Journal.PendingManualReview || len(result.Journal.RestorationRecordedFor) != 1 || result.Journal.RestorationRecordedFor[0] != experimentID) {
		runErr = errors.New("incomplete lifecycle or recovery journal: manual review required")
	}
	if runErr == nil {
		// Recheck all three namespaces and both exact witness names, not only
		// the one policy that was created in this run.
		readCtx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
		auditErr := finalAudit(readCtx)
		cancel()
		if auditErr != nil {
			runErr = fmt.Errorf("independent post-run lab inventory failed: %w", auditErr)
		}
	}
	result.RecoveryPending = runErr != nil
	if runErr != nil {
		// Keep the lock, but preserve a durable failure report for recovery.
		if err := writeExclusiveJSON(result.ReportPath, result); err != nil {
			return result, errors.Join(runErr, fmt.Errorf("failure report could not be saved: %w", err))
		}
		return result, runErr
	}
	// A pre-release report is durable before the lock is removed, but must
	// not falsely claim that the lock has already been released. After
	// successful release a second (exclusive) final report records success.
	pending := result.ReportPath + ".pending"
	if err := writeExclusiveJSON(pending, result); err != nil {
		return result, fmt.Errorf("verified run pending report could not be saved, lock retained: %w", err)
	}
	if err := os.Remove(result.LockPath); err != nil {
		return result, fmt.Errorf("verified run lock could not be released; pending report preserved: %w", err)
	}
	result.LockReleased = true
	result.RecoveryPending = false
	if err := writeExclusiveJSON(result.ReportPath, result); err != nil {
		return result, fmt.Errorf("lock released but final report could not be saved; inspect pending report and journal: %w", err)
	}
	// A stale .pending file is harmless. The finalized JSON is authoritative
	// for the final lock-release status and is never overwritten.
	_ = os.Remove(pending)
	return result, nil
}
