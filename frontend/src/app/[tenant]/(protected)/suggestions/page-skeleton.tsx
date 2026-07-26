"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type { FilterConfig } from "~/components/ui/page-header/types";
import { Skeleton } from "~/components/ui/skeleton";
import { STATUS_LABELS } from "~/lib/suggestions-helpers";

export function SuggestionSkeletons() {
  return (
    <div className="mt-4 space-y-4">
      {Array.from({ length: 3 }, (_, i) => (
        <div
          key={i}
          className="moto-content-surface rounded-3xl border p-5 shadow-sm"
        >
          <div className="flex flex-col gap-3 md:flex-row md:gap-4">
            <div className="hidden md:flex md:items-start md:pt-1">
              <Skeleton className="h-20 w-10 rounded-xl" />
            </div>
            <div className="min-w-0 flex-1 space-y-3">
              <div className="flex items-start justify-between gap-2">
                <Skeleton className="h-5 w-3/5 rounded" />
                <Skeleton className="h-5 w-16 rounded-full" />
              </div>
              <Skeleton className="h-4 w-full rounded" />
              <Skeleton className="h-4 w-4/5 rounded" />
              <div className="flex items-center gap-2 pt-1">
                <Skeleton className="h-5 w-5 rounded-full" />
                <Skeleton className="h-3 w-24 rounded" />
                <Skeleton className="h-3 w-20 rounded" />
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

// The page's isLoading branch renders the same PageHeaderWithSearch chrome
// (title, sort/status filters, search) it always renders, plus
// SuggestionSkeletons in place of the list. Route-level loading.tsx can't
// see the page's live filter state (nothing has mounted yet), so this uses
// the page's initial defaults (sortBy="score", statusFilter="all") with
// no-op handlers — the real component takes over once mounted.
function initialFilterConfigs(): FilterConfig[] {
  return [
    {
      id: "sort",
      label: "Sortierung",
      type: "buttons",
      value: "score",
      onChange: () => {},
      options: [
        { value: "score", label: "Beliebteste" },
        { value: "newest", label: "Neueste" },
      ],
    },
    {
      id: "status",
      label: "Status",
      type: "dropdown",
      value: "all",
      onChange: () => {},
      options: [
        { value: "all", label: "Alle Status" },
        ...Object.entries(STATUS_LABELS).map(([value, label]) => ({
          value,
          label,
        })),
      ],
    },
  ];
}

export function SuggestionsPageSkeleton() {
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Feedback"
        filters={initialFilterConfigs()}
        search={{
          value: "",
          onChange: () => {},
          placeholder: "Feedback durchsuchen...",
        }}
      />
      <SuggestionSkeletons />
    </div>
  );
}
