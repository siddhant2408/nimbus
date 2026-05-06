package handler

import (
	"encoding/json"
	"net/http"

	db "github.com/siddhant2408/nimbus/pkg/db/generated"
)

// Upper bound on free-text fields. `cloudWaitlistReasonMaxLen` is a
// product cap ("we don't need an essay for a waitlist"); the body-size
// cap further down is defense in depth against arbitrary storage
// abuse via the JSON body.
const (
	cloudWaitlistReasonMaxLen = 500

	// PatchOnboarding body is a tiny JSON with at most a 3-question
	// questionnaire. 16 KiB is ~10x the realistic ceiling — it's the
	// minimum that keeps the door open for future fields without
	// letting a malicious user stuff the JSONB column.
	patchOnboardingBodyLimit = 16 * 1024

	// Import payload contains the full starter-content template. Each
	// sub-issue's markdown description is ~2 KiB; with ~8 sub-issues,
	// a welcome issue (~3 KiB), and a project description, 64 KiB is
	// comfortably above realistic and still bounded.
	importStarterContentBodyLimit = 64 * 1024
)

// completeOnboardingRequest carries the client's view of which exit the
// user took from the flow. The client is the only place that knows
// whether Step 3's runtime connect was skipped, whether the cloud
// waitlist form was submitted, or whether Welcome's "I've done this
// before" path was used. Unknown/missing → OnboardingPathUnknown so
// legacy clients still complete the flow cleanly, just without a
// funnel-ready label.
type completeOnboardingRequest struct {
	CompletionPath string `json:"completion_path,omitempty"`
}

// CompleteOnboarding marks the authenticated user as having completed
// onboarding. Idempotent: the underlying query uses COALESCE so the
// original timestamp is preserved if called more than once.
//
// Emits `onboarding_completed` exactly once — the first call that
// actually flips `onboarded_at` from NULL. Subsequent calls are still
// 200 OK (for client-side retries) but skip the event so the funnel
// counts honest first-completion.
func (h *Handler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// Body is optional — an empty body is a legal legacy call.
	var req completeOnboardingRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	// Read the prior state so we can detect "was this call the one that
	// actually completed onboarding?" — MarkUserOnboarded uses COALESCE
	// and returns the preserved timestamp on repeat calls, which is not
	// the signal we need for the funnel.
	_, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	user, err := h.Queries.MarkUserOnboarded(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark onboarded")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

type patchOnboardingRequest struct {
	Questionnaire *json.RawMessage `json:"questionnaire,omitempty"`
}

// questionnaireAnswers mirrors the frontend's `QuestionnaireAnswers`
// shape. Only the first-time submission — every slot filled — is a
// funnel signal; partial saves are allowed but never emit.
type questionnaireAnswers struct {
	TeamSize      string `json:"team_size"`
	TeamSizeOther string `json:"team_size_other"`
	Role          string `json:"role"`
	RoleOther     string `json:"role_other"`
	UseCase       string `json:"use_case"`
	UseCaseOther  string `json:"use_case_other"`
}

func (q questionnaireAnswers) complete() bool {
	return q.TeamSize != "" && q.Role != "" && q.UseCase != ""
}

// PatchOnboarding persists the user's questionnaire answers. The
// field is optional; an omitted questionnaire is preserved. Which
// step the user is on is deliberately not persisted — every
// onboarding entry starts at Welcome.
//
// Emits `onboarding_questionnaire_submitted` exactly once per user:
// the first PATCH that transitions the answers from "at least one
// slot empty" to "all three filled". Revisions past that point don't
// re-emit — the funnel counts users, not edits.
func (h *Handler) PatchOnboarding(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	// Bound the body so the JSONB column can't be weaponized as bulk
	// storage — otherwise every subsequent `/api/me` read would have
	// to return the bloat.
	r.Body = http.MaxBytesReader(w, r.Body, patchOnboardingBodyLimit)
	var req patchOnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Read prior answers so we can detect the NULL/partial → complete
	// transition after the update. An errored decode on the prior row
	// is treated as "incomplete" — worst case we emit once more than
	// we should, never twice for the same transition.
	var before questionnaireAnswers
	if beforeUser, err := h.Queries.GetUser(r.Context(), parseUUID(userID)); err == nil {
		_ = json.Unmarshal(beforeUser.OnboardingQuestionnaire, &before)
	}

	params := db.PatchUserOnboardingParams{ID: parseUUID(userID)}
	if req.Questionnaire != nil {
		params.Questionnaire = []byte(*req.Questionnaire)
	}
	user, err := h.Queries.PatchUserOnboarding(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update onboarding")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

type joinCloudWaitlistRequest struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

// -----------------------------------------------------------------------------
// Starter content (post-onboarding opt-in)
// -----------------------------------------------------------------------------
//
// Users land in their workspace with starter_content_state=NULL and see
// a one-time dialog offering to seed example content. Two terminal
// transitions:
//
//   ImportStarterContent  NULL -> 'imported'  (also creates project, welcome
//                                              issue if agent-based, sub-issues,
//                                              pins — all in one transaction)
//   DismissStarterContent NULL -> 'dismissed'
//
// Why state-first, then seeding inside the same transaction:
//   - starter_content_state is the "have we asked / done this" bit, so it
//     must be set exactly once per user
//   - if we set state AFTER creation, a mid-request crash leaves duplicates
//     on retry (the original "Not idempotent" bug)
//   - if we set state BEFORE creation, a mid-request crash leaves the user
//     with 'imported' + no content
//   - inside a transaction, both commit together or neither does — and the
//     starting state check (must be NULL) guarantees the claim is atomic
//
// Content generation lives in TypeScript (the markdown templates are large
// and depend on the Q1–Q3 answers); the client POSTs the fully-rendered
// payload here, and the server's job is to (1) gate on state, (2) do the
// batch insert transactionally, (3) record the transition.

type importIssueSpec struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	// AssignToSelf: true for sub-issues (assigned to the current
	// user as a member). Server uses `user_id` per the app-wide
	// convention in AssigneePicker / resolveActor.
	AssignToSelf bool `json:"assign_to_self"`
}

// welcomeIssueTemplate is a PRE-rendered welcome issue — title +
// description + priority. There is no `agent_id` field on purpose:
// the server picks the target agent itself from ListAgents inside
// the transaction, so a stale or compromised client can't assign
// the welcome issue to an arbitrary agent.
type welcomeIssueTemplate struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	// Priority optional; defaults to "high" when empty.
	Priority string `json:"priority"`
}

type importStarterContentRequest struct {
	WorkspaceID string `json:"workspace_id"`

	Project struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"project"`

	// Welcome issue template — rendered regardless of branch. The
	// server creates it only when at least one agent exists in the
	// workspace; otherwise it's ignored.
	WelcomeIssueTemplate welcomeIssueTemplate `json:"welcome_issue_template"`

	// Both branches of sub-issues. The server picks which array to
	// seed based on whether the workspace has any agents at the
	// moment of the call — the client no longer decides. Sending
	// both is ~15 KB extra payload, which stays well under the
	// 64 KB MaxBytesReader cap above.
	AgentGuidedSubIssues []importIssueSpec `json:"agent_guided_sub_issues"`
	SelfServeSubIssues   []importIssueSpec `json:"self_serve_sub_issues"`
}

type importStarterContentResponse struct {
	User           UserResponse `json:"user"`
	ProjectID      string       `json:"project_id"`
	WelcomeIssueID *string      `json:"welcome_issue_id"`
}
