// file: internal/triagepoll/github_test.go
// version: 1.0.0
// guid: 8b1c2d3e-4f5a-6b7c-8d9e-0f1a2b3c4d5e

package triagepoll

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v84/github"
)

// fakeTriageAPI is a minimal stand-in for the GitHub issues API, following
// the pattern used in internal/ghops/mutate_test.go.
type fakeTriageAPI struct {
	addLabelsHandler http.HandlerFunc
	commentHandler   http.HandlerFunc

	// callOrder records "label" / "comment" in the order they were invoked,
	// so tests can assert WriteTriageResult's write ordering.
	callOrder []string
}

func (f *fakeTriageAPI) start(t *testing.T, ownerName string) (*github.Client, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+ownerName+"/issues/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/labels"):
			f.callOrder = append(f.callOrder, "label")
			if f.addLabelsHandler != nil {
				f.addLabelsHandler(w, r)
				return
			}
			writeJSON(t, w, []*github.Label{})
		case strings.HasSuffix(r.URL.Path, "/comments"):
			f.callOrder = append(f.callOrder, "comment")
			if f.commentHandler != nil {
				f.commentHandler(w, r)
				return
			}
			writeJSON(t, w, &github.IssueComment{})
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	client := github.NewClient(nil)
	u, _ := url.Parse(srv.URL + "/")
	client.BaseURL = u
	return client, srv.Close
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func testDecision() Decision {
	return Decision{
		IssueNumber:     42,
		Classification:  "NEEDS_REVIEW",
		Priority:        "P2",
		EstComplexity:   2,
		AffectedArea:    "dedup",
		SuggestedBranch: "fix/example",
		Reason:          "test",
	}
}

// TestWriteTriageResult_LabelsBeforeComment ensures the label write happens
// before the comment write. This ordering is what stops a persistent
// label-write failure from producing an unbounded stream of duplicate
// "triage complete" comments: if labeling fails, WriteTriageResult must
// return before ever posting a comment.
func TestWriteTriageResult_LabelsBeforeComment(t *testing.T) {
	api := &fakeTriageAPI{}
	client, closeFn := api.start(t, "hub-owner/hub-repo")
	defer closeFn()

	if err := WriteTriageResult(context.Background(), client, "hub-owner", "hub-repo", 42, testDecision()); err != nil {
		t.Fatalf("WriteTriageResult: %v", err)
	}

	if len(api.callOrder) != 2 || api.callOrder[0] != "label" || api.callOrder[1] != "comment" {
		t.Fatalf("expected [label, comment] call order, got %v", api.callOrder)
	}
}

// TestWriteTriageResult_LabelFailureSkipsComment is the core regression
// test: if AddLabelsToIssue fails, no comment should ever be posted. The
// old comment-first ordering meant every retry of a persistently-failing
// issue posted another duplicate comment while the label write kept
// failing — this test locks in that a label failure short-circuits before
// any comment call is made.
func TestWriteTriageResult_LabelFailureSkipsComment(t *testing.T) {
	api := &fakeTriageAPI{
		addLabelsHandler: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		},
	}
	client, closeFn := api.start(t, "hub-owner/hub-repo")
	defer closeFn()

	err := WriteTriageResult(context.Background(), client, "hub-owner", "hub-repo", 42, testDecision())
	if err == nil {
		t.Fatal("expected error when label write fails")
	}
	if len(api.callOrder) != 1 || api.callOrder[0] != "label" {
		t.Fatalf("expected only the label call to happen, got %v", api.callOrder)
	}
}

// TestFindUntriagedIssues_ExcludesTriageFailed ensures an issue already
// marked burndown:triage-failed is not resubmitted for triage every cycle.
func TestFindUntriagedIssues_ExcludesTriageFailed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/hub-owner/hub-repo/issues", func(w http.ResponseWriter, r *http.Request) {
		issues := []*github.Issue{
			{Number: github.Ptr(1), Title: github.Ptr("clean")},
			{
				Number: github.Ptr(2),
				Title:  github.Ptr("previously failed"),
				Labels: []*github.Label{{Name: github.Ptr(LabelTriageFailed)}},
			},
		}
		writeJSON(t, w, issues)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := github.NewClient(nil)
	u, _ := url.Parse(srv.URL + "/")
	client.BaseURL = u

	out, err := FindUntriagedIssues(context.Background(), client, "hub-owner", "hub-repo", "audiobook-organizer", "repo:")
	if err != nil {
		t.Fatalf("FindUntriagedIssues: %v", err)
	}
	if len(out) != 1 || out[0].Number != 1 {
		t.Fatalf("expected only issue #1 (triage-failed issue #2 excluded), got %+v", out)
	}
}
