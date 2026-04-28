import type { SnapshotRetrying } from "@/types/api";

interface RetryTableProps {
  entries: SnapshotRetrying[];
}

function relativeTime(dateStr: string): string {
  if (!dateStr) return "-";
  const date = new Date(dateStr);
  const now = Date.now();
  const diffMs = date.getTime() - now;
  if (diffMs <= 0) return "now";
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return `in ${diffSec}s`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `in ${diffMin}m`;
  const diffHr = Math.floor(diffMin / 60);
  return `in ${diffHr}h`;
}

export default function RetryTable({ entries }: RetryTableProps) {
  if (entries.length === 0) {
    return (
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-500">
        No retries queued
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
              <th className="px-4 py-3 font-medium text-gray-400 text-right">
                Attempt
              </th>
              <th className="px-4 py-3 font-medium text-gray-400">Due At</th>
              <th className="px-4 py-3 font-medium text-gray-400">Error</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800/50">
            {entries.map((entry) => (
              <tr
                key={`${entry.issue_id}-${entry.attempt}`}
                className="hover:bg-gray-800/30 transition-colors"
              >
                <td className="px-4 py-3">
                  <span className="font-mono text-xs text-gray-200">
                    {entry.issue_identifier}
                  </span>
                </td>
                <td className="px-4 py-3 text-right tabular-nums text-amber-400 font-medium">
                  {entry.attempt}
                </td>
                <td className="px-4 py-3">
                  <span className="text-gray-300 text-xs">
                    {relativeTime(entry.due_at)}
                  </span>
                  <div className="text-xs text-gray-600 font-mono">
                    {entry.due_at}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <span className="text-red-400 text-xs">{entry.error}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
