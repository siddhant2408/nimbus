import { useState } from "react";
import {
  usePersonasQuery,
  useAssignmentsQuery,
  useCreatePersonaMutation,
  useUpdatePersonaMutation,
  useDeletePersonaMutation,
} from "@/hooks/useQueries";
import type { PersonaListItem, PersonaOverrides } from "@/types/api";
import { fetchPersona } from "@/api/client";

// ---------- Form state ----------

interface PersonaFormData {
  name: string;
  description: string;
  prompt_template: string;
  overrides_json: string;
}

const EMPTY_FORM: PersonaFormData = {
  name: "",
  description: "",
  prompt_template: "",
  overrides_json: "{}",
};

function parseOverrides(json: string): PersonaOverrides {
  try {
    return JSON.parse(json) as PersonaOverrides;
  } catch {
    return {};
  }
}

// ---------- Sub-components ----------

function PersonaCard({
  persona,
  onEdit,
  onDelete,
}: {
  persona: PersonaListItem;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 flex flex-col gap-3">
      <div className="flex items-start justify-between">
        <div>
          <h4 className="font-medium text-white">{persona.name}</h4>
          {persona.description && (
            <p className="text-sm text-gray-400 mt-0.5">
              {persona.description}
            </p>
          )}
        </div>
        <div className="flex gap-2 shrink-0">
          <button
            onClick={onEdit}
            className="px-3 py-1.5 text-xs font-medium text-gray-300 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-lg transition-colors"
          >
            Edit
          </button>
          <button
            onClick={onDelete}
            className="px-3 py-1.5 text-xs font-medium text-red-400 bg-gray-800 hover:bg-red-950 border border-gray-700 hover:border-red-800 rounded-lg transition-colors"
          >
            Delete
          </button>
        </div>
      </div>
      <div className="flex items-center gap-3 text-xs text-gray-500">
        <span
          className={`inline-flex items-center gap-1 px-2 py-0.5 rounded ${persona.has_prompt ? "bg-emerald-950 text-emerald-400" : "bg-gray-800 text-gray-500"}`}
        >
          {persona.has_prompt ? "Has prompt" : "No prompt"}
        </span>
        {persona.overrides &&
          Object.keys(persona.overrides).length > 0 && (
            <span className="inline-flex items-center px-2 py-0.5 rounded bg-blue-950 text-blue-400">
              Has overrides
            </span>
          )}
      </div>
    </div>
  );
}

function PersonaForm({
  initial,
  isEdit,
  onSubmit,
  onCancel,
  isPending,
  error,
}: {
  initial: PersonaFormData;
  isEdit: boolean;
  onSubmit: (data: PersonaFormData) => void;
  onCancel: () => void;
  isPending: boolean;
  error: string | null;
}) {
  const [form, setForm] = useState<PersonaFormData>(initial);
  const [jsonError, setJsonError] = useState<string | null>(null);

  function handleOverridesChange(value: string) {
    setForm((f) => ({ ...f, overrides_json: value }));
    try {
      JSON.parse(value);
      setJsonError(null);
    } catch (e) {
      setJsonError(e instanceof Error ? e.message : "Invalid JSON");
    }
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (jsonError) return;
    onSubmit(form);
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="bg-gray-900 border border-gray-800 rounded-xl p-6 space-y-4"
    >
      <h3 className="text-lg font-medium text-white">
        {isEdit ? `Edit "${initial.name}"` : "Create New Persona"}
      </h3>

      {error && (
        <div className="bg-red-950/50 border border-red-900 rounded-lg p-3 text-sm text-red-300">
          {error}
        </div>
      )}

      <div>
        <label className="block text-sm font-medium text-gray-400 mb-1">
          Name
        </label>
        <input
          type="text"
          value={form.name}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          disabled={isEdit}
          placeholder="my-persona (lowercase, hyphens only)"
          pattern="^[a-z0-9-]+$"
          required
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-100 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-600 focus:border-transparent disabled:opacity-50"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-400 mb-1">
          Description
        </label>
        <input
          type="text"
          value={form.description}
          onChange={(e) =>
            setForm((f) => ({ ...f, description: e.target.value }))
          }
          placeholder="Brief description of this persona"
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-100 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-600 focus:border-transparent"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-400 mb-1">
          Prompt Template
        </label>
        <textarea
          value={form.prompt_template}
          onChange={(e) =>
            setForm((f) => ({ ...f, prompt_template: e.target.value }))
          }
          placeholder="Enter the prompt template for this persona..."
          rows={8}
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-100 placeholder-gray-500 font-mono focus:outline-none focus:ring-2 focus:ring-blue-600 focus:border-transparent resize-y"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-400 mb-1">
          Overrides (JSON)
        </label>
        <textarea
          value={form.overrides_json}
          onChange={(e) => handleOverridesChange(e.target.value)}
          rows={6}
          className={`w-full px-3 py-2 bg-gray-800 border rounded-lg text-sm text-gray-100 font-mono focus:outline-none focus:ring-2 focus:ring-blue-600 focus:border-transparent resize-y ${jsonError ? "border-red-700" : "border-gray-700"}`}
        />
        {jsonError && (
          <p className="text-xs text-red-400 mt-1">{jsonError}</p>
        )}
        <p className="text-xs text-gray-600 mt-1">
          Example: {`{"agent": {"max_turns": 10}, "codex": {"model": "o3"}}`}
        </p>
      </div>

      <div className="flex gap-3 pt-2">
        <button
          type="submit"
          disabled={isPending || !!jsonError}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg text-sm font-medium text-white transition-colors"
        >
          {isPending ? "Saving..." : isEdit ? "Update Persona" : "Create Persona"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-2 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-lg text-sm font-medium text-gray-300 transition-colors"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}

// ---------- Delete confirmation ----------

function DeleteConfirmation({
  name,
  onConfirm,
  onCancel,
  isPending,
}: {
  name: string;
  onConfirm: () => void;
  onCancel: () => void;
  isPending: boolean;
}) {
  return (
    <div className="bg-gray-900 border border-red-900/50 rounded-xl p-6">
      <h3 className="text-lg font-medium text-white mb-2">Delete Persona</h3>
      <p className="text-sm text-gray-400 mb-4">
        Are you sure you want to delete{" "}
        <span className="font-mono text-red-400">{name}</span>? This action
        cannot be undone.
      </p>
      <div className="flex gap-3">
        <button
          onClick={onConfirm}
          disabled={isPending}
          className="px-4 py-2 bg-red-600 hover:bg-red-700 disabled:opacity-50 rounded-lg text-sm font-medium text-white transition-colors"
        >
          {isPending ? "Deleting..." : "Delete"}
        </button>
        <button
          onClick={onCancel}
          className="px-4 py-2 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-lg text-sm font-medium text-gray-300 transition-colors"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

// ---------- Main component ----------

type View =
  | { kind: "list" }
  | { kind: "create" }
  | { kind: "edit"; name: string; initial: PersonaFormData }
  | { kind: "delete"; name: string };

export default function PersonaManager() {
  const [view, setView] = useState<View>({ kind: "list" });
  const [formError, setFormError] = useState<string | null>(null);

  const personasQuery = usePersonasQuery();
  const assignmentsQuery = useAssignmentsQuery();
  const createMutation = useCreatePersonaMutation();
  const updateMutation = useUpdatePersonaMutation();
  const deleteMutation = useDeletePersonaMutation();

  async function handleStartEdit(name: string) {
    try {
      const full = await fetchPersona(name);
      setView({
        kind: "edit",
        name,
        initial: {
          name: full.name,
          description: full.description,
          prompt_template: full.prompt_template ?? "",
          overrides_json: JSON.stringify(full.overrides ?? {}, null, 2),
        },
      });
      setFormError(null);
    } catch (err) {
      setFormError(
        err instanceof Error ? err.message : "Failed to load persona",
      );
    }
  }

  function handleCreate(data: PersonaFormData) {
    setFormError(null);
    createMutation.mutate(
      {
        name: data.name,
        description: data.description,
        prompt_template: data.prompt_template,
        overrides: parseOverrides(data.overrides_json),
      },
      {
        onSuccess: () => setView({ kind: "list" }),
        onError: (err) =>
          setFormError(
            err instanceof Error ? err.message : "Failed to create persona",
          ),
      },
    );
  }

  function handleUpdate(name: string, data: PersonaFormData) {
    setFormError(null);
    updateMutation.mutate(
      {
        name,
        persona: {
          name: data.name,
          description: data.description,
          prompt_template: data.prompt_template,
          overrides: parseOverrides(data.overrides_json),
        },
      },
      {
        onSuccess: () => setView({ kind: "list" }),
        onError: (err) =>
          setFormError(
            err instanceof Error ? err.message : "Failed to update persona",
          ),
      },
    );
  }

  function handleDelete(name: string) {
    deleteMutation.mutate(name, {
      onSuccess: () => setView({ kind: "list" }),
      onError: (err) =>
        setFormError(
          err instanceof Error ? err.message : "Failed to delete persona",
        ),
    });
  }

  // --- Loading / Error ---

  if (personasQuery.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-gray-500">Loading personas...</div>
      </div>
    );
  }

  if (personasQuery.isError) {
    return (
      <div className="bg-red-950/50 border border-red-900 rounded-xl p-6 text-red-300">
        <p className="font-medium">Failed to load personas</p>
        <p className="text-sm text-red-400 mt-1">
          {personasQuery.error instanceof Error
            ? personasQuery.error.message
            : "Unknown error"}
        </p>
      </div>
    );
  }

  const personas = personasQuery.data?.personas ?? [];
  const assignments = assignmentsQuery.data?.assignments ?? [];

  // --- Create view ---

  if (view.kind === "create") {
    return (
      <div className="space-y-6">
        <PersonaForm
          initial={EMPTY_FORM}
          isEdit={false}
          onSubmit={handleCreate}
          onCancel={() => {
            setView({ kind: "list" });
            setFormError(null);
          }}
          isPending={createMutation.isPending}
          error={formError}
        />
      </div>
    );
  }

  // --- Edit view ---

  if (view.kind === "edit") {
    return (
      <div className="space-y-6">
        <PersonaForm
          initial={view.initial}
          isEdit
          onSubmit={(data) => handleUpdate(view.name, data)}
          onCancel={() => {
            setView({ kind: "list" });
            setFormError(null);
          }}
          isPending={updateMutation.isPending}
          error={formError}
        />
      </div>
    );
  }

  // --- Delete confirmation ---

  if (view.kind === "delete") {
    return (
      <div className="space-y-6">
        <DeleteConfirmation
          name={view.name}
          onConfirm={() => handleDelete(view.name)}
          onCancel={() => {
            setView({ kind: "list" });
            setFormError(null);
          }}
          isPending={deleteMutation.isPending}
        />
      </div>
    );
  }

  // --- List view ---

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold text-white">Personas</h2>
          <p className="text-xs text-gray-500 mt-0.5">
            {personas.length} persona{personas.length !== 1 ? "s" : ""}{" "}
            configured
          </p>
        </div>
        <button
          onClick={() => {
            setView({ kind: "create" });
            setFormError(null);
          }}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium text-white transition-colors"
        >
          New Persona
        </button>
      </div>

      {formError && (
        <div className="bg-red-950/50 border border-red-900 rounded-lg p-3 text-sm text-red-300">
          {formError}
        </div>
      )}

      {/* Persona list */}
      {personas.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-500">
          No personas configured yet
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {personas.map((p) => (
            <PersonaCard
              key={p.name}
              persona={p}
              onEdit={() => handleStartEdit(p.name)}
              onDelete={() => setView({ kind: "delete", name: p.name })}
            />
          ))}
        </div>
      )}

      {/* Assignments */}
      <section>
        <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider mb-3">
          Current Assignments ({assignments.length})
        </h3>
        {assignments.length === 0 ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-500">
            No active assignments
          </div>
        ) : (
          <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-800 text-left">
                    <th className="px-4 py-3 font-medium text-gray-400">
                      Issue ID
                    </th>
                    <th className="px-4 py-3 font-medium text-gray-400">
                      Persona
                    </th>
                    <th className="px-4 py-3 font-medium text-gray-400">
                      Source
                    </th>
                    <th className="px-4 py-3 font-medium text-gray-400">
                      Assigned At
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800/50">
                  {assignments.map((a) => (
                    <tr
                      key={a.issue_id}
                      className="hover:bg-gray-800/30 transition-colors"
                    >
                      <td className="px-4 py-3 font-mono text-xs text-gray-200">
                        {a.issue_id}
                      </td>
                      <td className="px-4 py-3 text-gray-300">
                        {a.persona_name}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                            a.source === "label"
                              ? "bg-blue-950 text-blue-400"
                              : "bg-gray-800 text-gray-400"
                          }`}
                        >
                          {a.source}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-xs text-gray-500 font-mono">
                        {new Date(a.assigned_at).toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
