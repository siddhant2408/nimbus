import type { SnapshotRunning } from "@/types/api";

interface RunningTableProps {
  entries: SnapshotRunning[];
}

function formatNumber(n: number): string {
  return n.toLocaleString();
}

function relativeTime(dateStr: string | null): string {
  if (!dateStr) return "-";
  const date = new Date(dateStr);
  const now = Date.now();
  const diffMs = now - date.getTime();
  if (diffMs < 0) return "just now";
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  return `${diffHr}h ago`;
}

function StateBadge({ state }: { state: string }) {
  const lower = state.toLowerCase();
  let classes = "text-gray-300 bg-gray-800";
  if (lower.includes("running") || lower.includes("active")) {
    classes = "text-emerald-300 bg-emerald-950";
  } else if (lower.includes("wait") || lower.includes("pending")) {
    classes = "text-amber-300 bg-amber-950";
  } else if (lower.includes("error") || lower.includes("fail")) {
    classes = "text-red-300 bg-red-950";
  }
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${classes}`}
    >
      {state || "unknown"}
    </span>
  );
}

function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  return s.slice(0, max) + "...";
}

export default function RunningTable({ entries }: RunningTableProps) {
  if (entries.length === 0) {
    return (
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-500">
        No running sessions
      </div>
    );
  }

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-left">
              <th className="px-4 py-3 font-medium text-gray-400">Issue</th>
              <th className="px-4 py-3 font-medium text-gray-400">State</th>
              <th className="px-4 py-3 font-medium text-gray-400">Session</th>
              <th className="px-4 py-3 font-medium text-gray-400">Persona</th>
              <th className="px-4 py-3 font-medium text-gray-400 text-right">
                Turns
              </th>
              <th className="px-4 py-3 font-medium text-gray-400">
                Last Event
              </th>
              <th className="px-4 py-3 font-medium text-gray-400 text-right">
                Tokens
              </th>
              <th className="px-4 py-3 font-medium text-gray-400 text-right">
                Running
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800/50">
            {entries.map((entry) => (
              <tr
                key={entry.session_id || entry.issue_id}
                className="hover:bg-gray-800/30 transition-colors"
              >
                <td className="px-4 py-3">
                  <span className="font-mono text-xs text-gray-200">
                    {entry.issue_identifier}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <StateBadge state={entry.state} />
                </td>
                <td className="px-4 py-3">
                  <span className="font-mono text-xs text-gray-400">
                    {entry.session_id ? truncate(entry.session_id, 20) : "-"}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-300">
                  {entry.persona || (
                    <span className="text-gray-600">none</span>
                  )}
                </td>
                <td className="px-4 py-3 text-right tabular-nums text-gray-300">
                  {entry.turn_count}
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-col gap-0.5">
                    <span className="text-xs text-gray-400">
                      {entry.last_event || "-"}
                    </span>
                    {entry.last_message && (
                      <span className="text-xs text-gray-500 max-w-xs truncate block">
                        {truncate(entry.last_message, 60)}
                      </span>
                    )}
                    <span className="text-xs text-gray-600">
                      {relativeTime(entry.last_event_at)}
                    </span>
                  </div>
                </td>
                <td className="px-4 py-3 text-right">
                  <span className="tabular-nums text-gray-300">
                    {formatNumber(entry.tokens.total_tokens)}
                  </span>
                  <div className="text-xs text-gray-500 tabular-nums">
                    {formatNumber(entry.tokens.input_tokens)} /{" "}
                    {formatNumber(entry.tokens.output_tokens)}
                  </div>
                </td>
                <td className="px-4 py-3 text-right text-gray-400 text-xs">
                  {relativeTime(entry.started_at)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
