import type {
  StateResponse,
  RefreshResponse,
  IssueDetailResponse,
  PersonaListResponse,
  Persona,
  AssignmentsResponse,
} from "@/types/api";

const BASE = "/api/v1";

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options);
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const message =
      body?.error?.message ?? `Request failed: ${res.status} ${res.statusText}`;
    throw new Error(message);
  }
  return res.json() as Promise<T>;
}

export function fetchState(): Promise<StateResponse> {
  return request<StateResponse>(`${BASE}/state`);
}

export function fetchIssue(identifier: string): Promise<IssueDetailResponse> {
  return request<IssueDetailResponse>(`${BASE}/${encodeURIComponent(identifier)}`);
}

export function triggerRefresh(): Promise<RefreshResponse> {
  return request<RefreshResponse>(`${BASE}/refresh`, { method: "POST" });
}

export function fetchPersonas(): Promise<PersonaListResponse> {
  return request<PersonaListResponse>(`${BASE}/personas`);
}

export function fetchPersona(name: string): Promise<Persona> {
  return request<Persona>(`${BASE}/personas/${encodeURIComponent(name)}`);
}

export function createPersona(
  persona: Omit<Persona, "source_path">,
): Promise<Persona> {
  return request<Persona>(`${BASE}/personas`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(persona),
  });
}

export function updatePersona(
  name: string,
  persona: Omit<Persona, "source_path">,
): Promise<Persona> {
  return request<Persona>(`${BASE}/personas/${encodeURIComponent(name)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(persona),
  });
}

export function deletePersona(name: string): Promise<{ deleted: boolean }> {
  return request<{ deleted: boolean }>(
    `${BASE}/personas/${encodeURIComponent(name)}`,
    { method: "DELETE" },
  );
}

export function fetchAssignments(): Promise<AssignmentsResponse> {
  return request<AssignmentsResponse>(`${BASE}/personas/assignments`);
}
