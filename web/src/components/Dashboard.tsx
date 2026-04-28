import { useStateQuery, useRefreshMutation } from "@/hooks/useQueries";
import MetricsCards from "@/components/MetricsCards";
import RunningTable from "@/components/RunningTable";
import RetryTable from "@/components/RetryTable";

export default function Dashboard() {
  const { data, isLoading, isError, error, dataUpdatedAt } = useStateQuery();
  const refreshMutation = useRefreshMutation();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-gray-500">Loading state...</div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="bg-red-950/50 border border-red-900 rounded-xl p-6 text-red-300">
        <p className="font-medium">Failed to load state</p>
        <p className="text-sm text-red-400 mt-1">
          {error instanceof Error ? error.message : "Unknown error"}
        </p>
      </div>
    );
  }

  if (!data) return null;

  return (
    <div className="space-y-6">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold text-white">Dashboard</h2>
          <p className="text-xs text-gray-500 mt-0.5">
            Auto-refreshing every 2s
            {dataUpdatedAt > 0 && (
              <span>
                {" "}
                &middot; Last update{" "}
                {new Date(dataUpdatedAt).toLocaleTimeString()}
              </span>
            )}
          </p>
        </div>
        <button
          onClick={() => refreshMutation.mutate()}
          disabled={refreshMutation.isPending}
          className="px-4 py-2 bg-gray-800 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed border border-gray-700 rounded-lg text-sm font-medium text-gray-200 transition-colors"
        >
          {refreshMutation.isPending ? "Refreshing..." : "Refresh Now"}
        </button>
      </div>

      {/* Metrics */}
      <MetricsCards data={data} />

      {/* Running sessions */}
      <section>
        <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider mb-3">
          Running Sessions ({data.running?.length ?? 0})
        </h3>
        <RunningTable entries={data.running ?? []} />
      </section>

      {/* Retry queue */}
      <section>
        <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider mb-3">
          Retry Queue ({data.retrying?.length ?? 0})
        </h3>
        <RetryTable entries={data.retrying ?? []} />
      </section>

      {/* Rate limits */}
      <section>
        <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider mb-3">
          Rate Limits
        </h3>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          {data.rate_limits && Object.keys(data.rate_limits).length > 0 ? (
            <pre className="text-xs font-mono text-gray-300 whitespace-pre-wrap overflow-x-auto">
              {JSON.stringify(data.rate_limits, null, 2)}
            </pre>
          ) : (
            <p className="text-sm text-gray-500">
              No rate limit data available
            </p>
          )}
        </div>
      </section>
    </div>
  );
}
