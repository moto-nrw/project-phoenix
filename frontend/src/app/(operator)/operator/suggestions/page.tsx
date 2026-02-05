"use client";

import { useState, useMemo, useCallback } from "react";
import { useRouter } from "next/navigation";
import { AnimatePresence, LayoutGroup, motion } from "framer-motion";
import useSWR from "swr";
import {
  PageHeaderWithSearch,
  type FilterConfig,
} from "~/components/ui/page-header";
import { Skeleton } from "~/components/ui/skeleton";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorSuggestionsService } from "~/lib/operator/suggestions-api";
import {
  OPERATOR_STATUS_LABELS,
  OPERATOR_STATUS_STYLES,
} from "~/lib/operator/suggestions-helpers";
import type { OperatorSuggestionStatus } from "~/lib/operator/suggestions-helpers";

function getRelativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "gerade eben";
  if (minutes < 60)
    return `vor ${minutes} ${minutes === 1 ? "Minute" : "Minuten"}`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `vor ${hours} ${hours === 1 ? "Stunde" : "Stunden"}`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `vor ${days} ${days === 1 ? "Tag" : "Tagen"}`;
  const weeks = Math.floor(days / 7);
  if (weeks < 5) return `vor ${weeks} ${weeks === 1 ? "Woche" : "Wochen"}`;
  const months = Math.floor(days / 30);
  if (months < 12) return `vor ${months} ${months === 1 ? "Monat" : "Monaten"}`;
  const years = Math.floor(days / 365);
  return `vor ${years} ${years === 1 ? "Jahr" : "Jahren"}`;
}

function getInitials(name: string): string {
  const parts = name.split(" ").filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return (parts[0]?.[0] ?? "?").toUpperCase();
  return (
    (parts[0]?.[0] ?? "").toUpperCase() +
    (parts.at(-1)?.[0] ?? "").toUpperCase()
  );
}

export default function OperatorSuggestionsPage() {
  const router = useRouter();
  const { isAuthenticated } = useOperatorAuth();
  useSetBreadcrumb({ pageTitle: "Vorschläge" });
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [statusUpdating, setStatusUpdating] = useState<string | null>(null);

  const {
    data: suggestions,
    isLoading,
    mutate,
  } = useSWR(isAuthenticated ? "operator-suggestions" : null, () =>
    operatorSuggestionsService.fetchAll(),
  );

  const filteredSuggestions = useMemo(() => {
    if (!suggestions) return [];
    let result = suggestions;
    if (statusFilter !== "all") {
      result = result.filter((s) => s.status === statusFilter);
    }
    if (searchTerm.trim()) {
      const term = searchTerm.toLowerCase();
      result = result.filter(
        (s) =>
          s.title.toLowerCase().includes(term) ||
          s.description.toLowerCase().includes(term) ||
          s.authorName.toLowerCase().includes(term),
      );
    }
    return result;
  }, [suggestions, searchTerm, statusFilter]);

  const handleStatusChange = useCallback(
    async (id: string, newStatus: OperatorSuggestionStatus) => {
      setStatusUpdating(id);
      try {
        const updated = await operatorSuggestionsService.updateStatus(
          id,
          newStatus,
        );
        await mutate(
          (current) => current?.map((s) => (s.id === updated.id ? updated : s)),
          { revalidate: false },
        );
      } catch (error) {
        console.error("Failed to update status:", error);
      } finally {
        setStatusUpdating(null);
      }
    },
    [mutate],
  );

  const filterConfigs: FilterConfig[] = [
    {
      id: "status",
      label: "Status",
      type: "dropdown",
      value: statusFilter,
      onChange: (value) => setStatusFilter(value as string),
      options: [
        { value: "all", label: "Alle Status" },
        ...Object.entries(OPERATOR_STATUS_LABELS).map(([value, label]) => ({
          value,
          label,
        })),
      ],
    },
  ];

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Vorschläge"
        badge={
          suggestions
            ? {
                count: suggestions.length,
                label: suggestions.length === 1 ? "Beitrag" : "Beiträge",
              }
            : undefined
        }
        filters={filterConfigs}
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Vorschläge durchsuchen...",
        }}
      />

      {isLoading && <SuggestionSkeletons />}
      {!isLoading && filteredSuggestions.length === 0 && (
        <div className="flex flex-col items-center gap-3 py-12 text-center">
          <svg
            className="h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
            />
          </svg>
          <p className="text-lg font-medium text-gray-900">
            {searchTerm.trim()
              ? "Keine Ergebnisse gefunden"
              : "Keine Vorschläge vorhanden"}
          </p>
          <p className="text-sm text-gray-500">
            {searchTerm.trim()
              ? "Versuche einen anderen Suchbegriff."
              : "Es wurden noch keine Vorschläge eingereicht."}
          </p>
        </div>
      )}
      {!isLoading && filteredSuggestions.length > 0 && (
        <LayoutGroup>
          <div className="mt-4 space-y-4">
            <AnimatePresence>
              {filteredSuggestions.map((suggestion) => (
                <motion.div
                  key={suggestion.id}
                  layout
                  transition={{ type: "spring", stiffness: 500, damping: 35 }}
                >
                  <button
                    type="button"
                    onClick={() =>
                      router.push(`/operator/suggestions/${suggestion.id}`)
                    }
                    className="w-full text-left"
                  >
                    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150 md:hover:-translate-y-0.5 md:hover:border-blue-200/50 md:hover:shadow-[0_12px_40px_rgb(0,0,0,0.18)]">
                      <div className="flex items-start justify-between gap-2">
                        <h3 className="text-base font-semibold text-gray-900">
                          {suggestion.title}
                        </h3>
                        <div className="flex shrink-0 items-center gap-2">
                          <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                            {suggestion.score} Stimmen
                          </span>
                          <select
                            value={suggestion.status}
                            onClick={(e) => e.stopPropagation()}
                            onChange={(e) => {
                              e.stopPropagation();
                              void handleStatusChange(
                                suggestion.id,
                                e.target.value as OperatorSuggestionStatus,
                              );
                            }}
                            disabled={statusUpdating === suggestion.id}
                            className={`rounded-full border-0 px-2.5 py-0.5 text-xs font-medium ${OPERATOR_STATUS_STYLES[suggestion.status]} cursor-pointer focus:ring-2 focus:ring-blue-500 focus:outline-none disabled:opacity-50`}
                          >
                            {Object.entries(OPERATOR_STATUS_LABELS).map(
                              ([value, label]) => (
                                <option key={value} value={value}>
                                  {label}
                                </option>
                              ),
                            )}
                          </select>
                        </div>
                      </div>
                      <p className="mt-1 line-clamp-2 text-sm text-gray-600">
                        {suggestion.description}
                      </p>
                      <div className="mt-3 flex items-center gap-2 text-xs text-gray-500">
                        <span className="flex h-5 w-5 items-center justify-center rounded-full bg-blue-100 text-[10px] font-medium text-blue-700">
                          {getInitials(suggestion.authorName)}
                        </span>
                        <span>{suggestion.authorName}</span>
                        <span>·</span>
                        <span>{getRelativeTime(suggestion.createdAt)}</span>
                        {suggestion.commentCount > 0 && (
                          <>
                            <span>·</span>
                            <span>
                              {suggestion.commentCount}{" "}
                              {suggestion.commentCount === 1
                                ? "Kommentar"
                                : "Kommentare"}
                            </span>
                          </>
                        )}
                      </div>
                    </div>
                  </button>
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        </LayoutGroup>
      )}
    </div>
  );
}

function SuggestionSkeletons() {
  return (
    <div className="mt-4 space-y-4">
      {Array.from({ length: 3 }, (_, i) => (
        <div
          key={i}
          className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]"
        >
          <div className="space-y-3">
            <div className="flex items-start justify-between gap-2">
              <Skeleton className="h-5 w-3/5 rounded" />
              <Skeleton className="h-5 w-20 rounded-full" />
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
      ))}
    </div>
  );
}
