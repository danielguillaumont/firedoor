package attribution

// Append-only write-ahead recovery journal for the exact Firedoor v0.4 lab.
// This file only performs local filesystem operations. Its driver wrapper
// records intent BEFORE an underlying Kubernetes create operation is called.
// The journal is not authenticated evidence and cannot guarantee recovery
// after an OS crash or power loss; live execution remains a separate slice.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const RecoveryJournalSchema = "firedoor.v04.recovery-journal.v1"
const maxJournalBytes = 1024 * 1024

type RecoveryEvent struct {
	Schema       string    `json:"schema"`
	Sequence     int       `json:"sequence"`
	CapturedUTC  time.Time `json:"captured_utc"`
	Context      string    `json:"context"`
	ExperimentID string    `json:"experiment_id"`
	Namespace    string    `json:"namespace"`
	PolicyName   string    `json:"policy_name"`
	Event        string    `json:"event"`
	UID          string    `json:"uid,omitempty"`
}

type RecoverySummary struct {
	Schema                 string   `json:"schema"`
	Context                string   `json:"context"`
	EventCount             int      `json:"event_count"`
	PendingManualReview    bool     `json:"pending_manual_review"`
	UnresolvedExperiments  []string `json:"unresolved_experiments"`
	RestorationRecordedFor []string `json:"restoration_recorded_for"`
	Limitations            []string `json:"limitations"`
}

// RecoveryJournal owns a newly created file. NewRecoveryJournal refuses to
// overwrite or append to previous runs, including files with no contents.
// A later live entrypoint must use a unique, Git-ignored path per run.
type RecoveryJournal struct {
	mu   sync.Mutex
	path string
}

func NewRecoveryJournal(path string) (*RecoveryJournal, error) {
	if path == "" {
		return nil, errors.New("journal path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("refusing to overwrite existing recovery journal: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("new journal sync failed: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return &RecoveryJournal{path: path}, nil
}

func journalApproved(ex Experiment) error { return approvedExperiment(ex) }

func eventUIDRules(kind, uid string) error {
	switch kind {
	case "create_intent", "create_uncertain":
		if uid != "" {
			return errors.New("intent or unknown outcome must not invent a UID")
		}
	case "created_uid", "delete_intent", "delete_accepted", "absent_verified", "restoration_observed":
		if !validUID(uid) {
			return errors.New("event requires a valid server UID")
		}
	default:
		return errors.New("unsupported recovery event")
	}
	return nil
}

func validateEvent(ev RecoveryEvent) error {
	if ev.Schema != RecoveryJournalSchema || ev.Context != OwnedContext || ev.Sequence <= 0 || ev.CapturedUTC.IsZero() ||
		!ev.CapturedUTC.Equal(ev.CapturedUTC.UTC()) {
		return errors.New("invalid recovery event schema/context/sequence/time")
	}
	if err := eventUIDRules(ev.Event, ev.UID); err != nil {
		return err
	}
	for _, ex := range ApprovedPlan().Experiments {
		if ev.ExperimentID == ex.ID && ev.Namespace == ex.Witness.PolicyNamespace && ev.PolicyName == ex.Witness.PolicyName {
			return nil
		}
	}
	return errors.New("event references unapproved experiment, namespace, or policy")
}

// journalState ensures events cannot disguise an unresolved or incomplete
// mutation as a confirmed restoration. An empty file is an initial state.
type journalState struct {
	stage string
	uid   string
}

func validateJournalSequence(events []RecoveryEvent) error {
	states := make(map[string]journalState)
	for i, ev := range events {
		if err := validateEvent(ev); err != nil {
			return fmt.Errorf("event %d: %w", i+1, err)
		}
		if ev.Sequence != i+1 {
			return errors.New("non-contiguous journal event sequence")
		}
		s := states[ev.ExperimentID]
		switch ev.Event {
		case "create_intent":
			if s.stage != "" {
				return errors.New("duplicate or restarted experiment intent")
			}
			s.stage = "intent"
		case "created_uid":
			if s.stage != "intent" {
				return errors.New("UID recorded without fresh intent")
			}
			s.uid, s.stage = ev.UID, "created"
		case "create_uncertain":
			if s.stage != "intent" {
				return errors.New("uncertain result without intent")
			}
			s.stage = "uncertain"
		case "delete_intent":
			if s.stage != "created" || s.uid != ev.UID {
				return errors.New("delete intent without identical created UID")
			}
			s.stage = "deleting"
		case "delete_accepted":
			if s.stage != "deleting" || s.uid != ev.UID {
				return errors.New("delete acknowledgement without identical UID")
			}
			s.stage = "delete_accepted"
		case "absent_verified":
			// The policy might already be absent following a failed DELETE reply.
			if (s.stage != "created" && s.stage != "deleting" && s.stage != "delete_accepted") || s.uid != ev.UID {
				return errors.New("absence recorded without matching created UID")
			}
			s.stage = "absent"
		case "restoration_observed":
			if s.stage != "absent" || s.uid != ev.UID {
				return errors.New("restoration recorded before absence verified")
			}
			s.stage = "restored"
		}
		states[ev.ExperimentID] = s
	}
	return nil
}

func readRecoveryEvents(path string) ([]RecoveryEvent, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxJournalBytes {
		return nil, errors.New("recovery journal is not a bounded regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		return nil, errors.New("incomplete trailing recovery event: manual review required")
	}
	var events []RecoveryEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		var ev RecoveryEvent
		if err := dec.Decode(&ev); err != nil {
			return nil, fmt.Errorf("invalid journal event JSON: %w", err)
		}
		// Reject trailing JSON after an otherwise valid object.
		var extraneous any
		if err := dec.Decode(&extraneous); err != io.EOF {
			return nil, errors.New("additional journal JSON after event")
		}
		events = append(events, ev)
		if len(events) > 100 {
			return nil, errors.New("journal has too many events")
		}
	}
	if err := validateJournalSequence(events); err != nil {
		return nil, err
	}
	return events, nil
}

// AppendEvent re-reads and validates the entire bounded log before appending.
// A failed write or sync MUST prevent live creation when it precedes create.
func (j *RecoveryJournal) AppendEvent(ex Experiment, kind, uid string) error {
	if j == nil {
		return errors.New("missing recovery journal")
	}
	if err := journalApproved(ex); err != nil {
		return err
	}
	if err := eventUIDRules(kind, uid); err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	events, err := readRecoveryEvents(j.path)
	if err != nil {
		return fmt.Errorf("journal pre-append validation failed: %w", err)
	}
	ev := RecoveryEvent{Schema: RecoveryJournalSchema, Sequence: len(events) + 1,
		CapturedUTC: time.Now().UTC(), Context: OwnedContext, ExperimentID: ex.ID,
		Namespace: ex.Witness.PolicyNamespace, PolicyName: ex.Witness.PolicyName, Event: kind, UID: uid}
	if err := validateJournalSequence(append(events, ev)); err != nil {
		return err
	}
	blob, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(j.path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(blob, '\n')); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("recovery event not durably confirmed: %w", err)
	}
	return nil
}

// InspectRecoveryJournal reads local evidence only. Successful parsing is not
// proof that Kubernetes policies are absent or the experiment completed.
func InspectRecoveryJournal(path string) (RecoverySummary, error) {
	sum := RecoverySummary{Schema: RecoveryJournalSchema, Context: OwnedContext,
		UnresolvedExperiments: []string{}, RestorationRecordedFor: []string{},
		Limitations: []string{"Local journal is unauthenticated and must not be treated as cluster state.",
			"An interrupted write, power loss or incomplete log requires manual recovery review.",
			"Read-only Kubernetes witness inventory must independently confirm absence."}}
	events, err := readRecoveryEvents(path)
	if err != nil {
		sum.PendingManualReview = true
		return sum, err
	}
	sum.EventCount = len(events)
	states := make(map[string]string)
	for _, ev := range events {
		states[ev.ExperimentID] = ev.Event
	}
	for _, ex := range ApprovedPlan().Experiments {
		last, present := states[ex.ID]
		if !present {
			continue
		}
		if last == "restoration_observed" {
			sum.RestorationRecordedFor = append(sum.RestorationRecordedFor, ex.ID)
		} else {
			sum.PendingManualReview = true
			sum.UnresolvedExperiments = append(sum.UnresolvedExperiments, ex.ID)
		}
	}
	return sum, nil
}

// JournaledLifecycleDriver wraps the existing driver but never instantiates a
// real Kubernetes writer. Tests inject synthetic drivers. A later live runner
// must construct it ONLY after explicit operator consent.
type JournaledLifecycleDriver struct {
	Inner   LifecycleDriver
	Journal *RecoveryJournal
	uid     map[string]string
	absent  map[string]bool
}

func NewJournaledLifecycleDriver(inner LifecycleDriver, journal *RecoveryJournal) (*JournaledLifecycleDriver, error) {
	if inner == nil || journal == nil {
		return nil, errors.New("journal and delegate driver required")
	}
	return &JournaledLifecycleDriver{Inner: inner, Journal: journal, uid: map[string]string{}, absent: map[string]bool{}}, nil
}
func (d *JournaledLifecycleDriver) Preflight(ctx context.Context) error {
	return d.Inner.Preflight(ctx)
}
func (d *JournaledLifecycleDriver) Inspect(ctx context.Context, ex Experiment, uid string) error {
	return d.Inner.Inspect(ctx, ex, uid)
}
func (d *JournaledLifecycleDriver) Probe(ctx context.Context, ex Experiment, phase Phase) (PhaseObservation, error) {
	got, err := d.Inner.Probe(ctx, ex, phase)
	if err != nil {
		return got, err
	}
	if phase == Restored && got.Observed == Unreachable && got.AttemptCount == 2 && got.Reason != "" {
		uid := d.uid[ex.ID]
		if !validUID(uid) {
			return got, errors.New("restoration recorded without known server UID")
		}
		if err := d.Journal.AppendEvent(ex, "restoration_observed", uid); err != nil {
			return got, err
		}
	}
	return got, nil
}
func (d *JournaledLifecycleDriver) Create(ctx context.Context, ex Experiment, obj WitnessObject) (string, error) {
	if err := d.Journal.AppendEvent(ex, "create_intent", ""); err != nil {
		return "", fmt.Errorf("write-ahead intent refused creation: %w", err)
	}
	uid, createErr := d.Inner.Create(ctx, ex, obj)
	if validUID(uid) {
		d.uid[ex.ID] = uid
		if err := d.Journal.AppendEvent(ex, "created_uid", uid); err != nil {
			return uid, errors.Join(createErr, fmt.Errorf("UID recovery record failed: %w", err))
		}
	} else if err := d.Journal.AppendEvent(ex, "create_uncertain", ""); err != nil {
		return uid, errors.Join(createErr, fmt.Errorf("uncertain creation record failed: %w", err))
	}
	return uid, createErr
}
func (d *JournaledLifecycleDriver) Delete(ctx context.Context, ex Experiment, uid string) error {
	if err := d.Journal.AppendEvent(ex, "delete_intent", uid); err != nil {
		return fmt.Errorf("delete intent not recorded: %w", err)
	}
	if err := d.Inner.Delete(ctx, ex, uid); err != nil {
		return err
	}
	return d.Journal.AppendEvent(ex, "delete_accepted", uid)
}
func (d *JournaledLifecycleDriver) VerifyGone(ctx context.Context, ex Experiment) error {
	if err := d.Inner.VerifyGone(ctx, ex); err != nil {
		return err
	}
	uid := d.uid[ex.ID]
	if !validUID(uid) {
		return errors.New("absence verification without known UID")
	}
	if d.absent[ex.ID] {
		return nil
	}
	if err := d.Journal.AppendEvent(ex, "absent_verified", uid); err != nil {
		return err
	}
	d.absent[ex.ID] = true
	return nil
}
