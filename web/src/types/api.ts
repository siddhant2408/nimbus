// Matches domain.Snapshot from Go backend.
export interface StateResponse {
  generated_at: string;
  counts: SnapshotCounts;
  running: SnapshotRunning[];
  retrying: SnapshotRetrying[];
  codex_totals: SnapshotCodexTotals;
  rate_limits: Record<string, unknown> | null;
}

export interface SnapshotCounts {
  running: number;
  retrying: number;
}

export interface SnapshotRunning {
  issue_id: string;
  issue_identifier: string;
  state: string;
  session_id: string;
  turn_count: number;
  persona: string;
  last_event: string;
  last_message: string;
  started_at: string;
  last_event_at: string | null;
  tokens: SnapshotTokens;
  worker_host?: string;
  workspace_path?: string;
}

export interface SnapshotRetrying {
  issue_id: string;
  issue_identifier: string;
  attempt: number;
  due_at: string;
  error: string;
  worker_host?: string;
}

export interface SnapshotTokens {
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
}

export interface SnapshotCodexTotals {
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  seconds_running: number;
}

// Matches domain.RefreshResponse.
export interface RefreshResponse {
  queued: boolean;
  coalesced: boolean;
  requested_at: string;
}

// Matches persona.Persona from Go backend.
export interface Persona {
  name: string;
  description: string;
  source_path: string;
  overrides: PersonaOverrides;
  prompt_template?: string;
  has_prompt?: boolean;
}

export interface PersonaOverrides {
  agent?: AgentOverrides;
  codex?: CodexOverrides;
  tools?: ToolOverrides;
}

export interface AgentOverrides {
  max_turns?: number;
}

export interface CodexOverrides {
  approval_policy?: string;
  model?: string;
  turn_timeout_ms?: number;
}

export interface ToolOverrides {
  allow?: string[];
  deny?: string[];
}

export interface PersonaListResponse {
  personas: PersonaListItem[];
}

export interface PersonaListItem {
  name: string;
  description: string;
  source_path: string;
  overrides: PersonaOverrides;
  has_prompt: boolean;
}

// Matches persona.Assignment from Go backend.
export interface PersonaAssignment {
  issue_id: string;
  persona_name: string;
  assigned_at: string;
  source: string;
}

export interface AssignmentsResponse {
  assignments: PersonaAssignment[];
}

export interface IssueDetailResponse {
  issue_identifier: string;
  issue_id: string;
  status: string;
  workspace?: {
    path: string;
    host: string;
  };
  running?: {
    session_id: string;
    turn_count: number;
    state: string;
    persona: string;
    started_at: string;
    last_event: string;
    last_message: string;
    last_event_at: string | null;
    tokens: SnapshotTokens;
  };
  retry?: {
    attempt: number;
    due_at: string;
    error: string;
  };
}

export interface ApiError {
  error: {
    code: string;
    message: string;
  };
}
