// Refresh button for header
// Triggers SWR cache revalidation to refresh all page data

"use client";

import { useState, useCallback } from "react";
import { RotateCw } from "lucide-react";
import { useSWRConfig } from "swr";

export function RefreshButton() {
  const [isRefreshing, setIsRefreshing] = useState(false);
  const { mutate } = useSWRConfig();

  const handleRefresh = useCallback(async () => {
    if (isRefreshing) return;

    setIsRefreshing(true);
    try {
      // Revalidate all SWR caches
      await mutate(() => true, undefined, { revalidate: true });
    } finally {
      // Keep the animation for at least 600ms for visual feedback
      setTimeout(() => setIsRefreshing(false), 600);
    }
  }, [isRefreshing, mutate]);

  return (
    <button
      type="button"
      onClick={handleRefresh}
      disabled={isRefreshing}
      aria-label="Daten aktualisieren"
      title="Daten aktualisieren"
      className="flex h-10 w-10 items-center justify-center rounded-lg text-gray-500 transition-colors duration-200 hover:bg-gray-100 hover:text-gray-700 disabled:opacity-50"
    >
      <RotateCw className={`h-5 w-5 ${isRefreshing ? "animate-spin" : ""}`} />
    </button>
  );
}
