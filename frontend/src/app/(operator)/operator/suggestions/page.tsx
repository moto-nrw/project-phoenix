"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { AnimatePresence, LayoutGroup, motion } from "framer-motion";
import { ThumbsUp, ThumbsDown } from "lucide-react";
import useSWR from "swr";
import {
  PageHeaderWithSearch,
  type FilterConfig,
} from "~/components/ui/page-header";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDropdown } from "~/components/operator/status-dropdown";
import { OperatorCommentAccordion } from "~/components/operator/operator-comment-accordion";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorSuggestionsService } from "~/lib/operator/suggestions-api";
import { OPERATOR_STATUS_LABELS } from "~/lib/operator/suggestions-helpers";
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
  const { isAuthenticated } = useOperatorAuth();
  useSetBreadcrumb({ pageTitle: "Feedback" });
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [statusUpdating, setStatusUpdating] = useState<string | null>(null);
  const [openDropdownId, setOpenDropdownId] = useState<string | null>(null);

  const {
    data: suggestions,
    isLoading,
    mutate,
  } = useSWR(
    isAuthenticated ? "operator-suggestions" : null,
    () => operatorSuggestionsService.fetchAll(),
    { refreshInterval: 30000 }, // Refresh every 30 seconds to catch new posts
  );

  // Track previous counts to detect external changes
  const prevCountsRef = useRef<{ unviewed: number; unread: number } | null>(
    null,
  );

  // Sync sidebar badge when suggestion data changes (e.g., new posts from users)
  useEffect(() => {
    if (!suggestions) return;

    const unviewedCount = suggestions.filter((s) => s.isNew).length;
    const unreadCount = suggestions.reduce((sum, s) => sum + s.unreadCount, 0);

    // Compare with previous counts
    if (prevCountsRef.current !== null) {
      const { unviewed: prevUnviewed, unread: prevUnread } =
        prevCountsRef.current;
      if (unviewedCount !== prevUnviewed || unreadCount !== prevUnread) {
        // Counts changed - dispatch events to update sidebar
        window.dispatchEvent(
          new CustomEvent("operator-suggestions-unviewed-refresh"),
        );
      }
    }

    prevCountsRef.current = { unviewed: unviewedCount, unread: unreadCount };
  }, [suggestions]);

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
        await operatorSuggestionsService.updateStatus(id, newStatus);
        // Backend marks post as viewed when changing status, update local state
        await mutate(
          (current) =>
            current?.map((s) =>
              s.id === id ? { ...s, status: newStatus, isNew: false } : s,
            ),
          { revalidate: false },
        );
        // Notify sidebar to refresh unviewed count
        window.dispatchEvent(
          new CustomEvent("operator-suggestions-unviewed-refresh"),
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
        title="Feedback"
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
          placeholder: "Feedback durchsuchen...",
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
              : "Kein Feedback vorhanden"}
          </p>
          <p className="text-sm text-gray-500">
            {searchTerm.trim()
              ? "Versuche einen anderen Suchbegriff."
              : "Es wurde noch kein Feedback eingereicht."}
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
                  className={`relative ${openDropdownId === suggestion.id ? "z-10" : "z-0"}`}
                >
                  <div className="rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
                    <div className="p-5">
                      <div className="flex items-start justify-between gap-2">
                        <h3 className="flex items-center gap-2 text-base font-semibold text-gray-900">
                          {suggestion.title}
                          {suggestion.isNew && (
                            <span className="rounded-full bg-blue-500 px-2 py-0.5 text-[10px] font-semibold text-white">
                              Neu
                            </span>
                          )}
                        </h3>
                        <div className="flex shrink-0 items-center gap-2">
                          <div className="flex items-center gap-2">
                            <span className="flex items-center gap-1 text-[#83CD2D]">
                              <ThumbsUp
                                className="h-4 w-4"
                                fill="currentColor"
                              />
                              <span className="text-xs font-bold">
                                {suggestion.upvotes}
                              </span>
                            </span>
                            <span className="flex items-center gap-1 text-red-500">
                              <ThumbsDown
                                className="h-4 w-4"
                                fill="currentColor"
                              />
                              <span className="text-xs font-bold">
                                {suggestion.downvotes}
                              </span>
                            </span>
                          </div>
                          <StatusDropdown
                            value={suggestion.status}
                            onChange={(newStatus) =>
                              void handleStatusChange(suggestion.id, newStatus)
                            }
                            disabled={statusUpdating === suggestion.id}
                            onOpenChange={(open) =>
                              setOpenDropdownId(open ? suggestion.id : null)
                            }
                          />
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
                      </div>
                    </div>
                    <OperatorCommentAccordion
                      postId={suggestion.id}
                      commentCount={suggestion.commentCount}
                      unreadCount={suggestion.unreadCount}
                      isNew={suggestion.isNew}
                    />
                  </div>
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
