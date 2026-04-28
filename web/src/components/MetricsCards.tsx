import type { StateResponse } from "@/types/api";

interface MetricsCardsProps {
  data: StateResponse;
}

function formatNumber(n: number): string {
  return n.toLocaleString();
}

function formatRuntime(seconds: number): string {
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  if (seconds < 3600) return `${(seconds / 60).toFixed(1)}m`;
  return `${(seconds / 3600).toFixed(1)}h`;
}

interface CardProps {
  title: string;
  value: string;
  subtitle?: string;
  color: string;
}

function Card({ title, value, subtitle, color }: CardProps) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
      <p className="text-sm font-medium text-gray-400 mb-1">{title}</p>
      <p className={`text-3xl font-bold tabular-nums ${color}`}>{value}</p>
      {subtitle && (
        <p className="text-xs text-gray-500 mt-1.5">{subtitle}</p>
      )}
    </div>
  );
}

export default function MetricsCards({ data }: MetricsCardsProps) {
  const { counts, codex_totals } = data;

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <Card
        title="Running Sessions"
        value={String(counts.running)}
        color="text-emerald-400"
      />
      <Card
        title="Retrying"
        value={String(counts.retrying)}
        color={counts.retrying > 0 ? "text-amber-400" : "text-gray-300"}
      />
      <Card
        title="Total Tokens"
        value={formatNumber(codex_totals.total_tokens)}
        subtitle={`${formatNumber(codex_totals.input_tokens)} in / ${formatNumber(codex_totals.output_tokens)} out`}
        color="text-blue-400"
      />
      <Card
        title="Runtime"
        value={formatRuntime(codex_totals.seconds_running)}
        subtitle={`${codex_totals.seconds_running.toFixed(1)} seconds total`}
        color="text-purple-400"
      />
    </div>
  );
}
