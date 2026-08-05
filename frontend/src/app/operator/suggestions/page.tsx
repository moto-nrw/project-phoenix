"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import Link from "next/link";
import { AnimatePresence, LayoutGroup, motion } from "framer-motion";
import { ThumbsUp, ThumbsDown } from "lucide-react";
// eslint-disable-next-line no-restricted-imports -- operator pages use useOperatorAuth, not NextAuth
import useSWR from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type { FilterConfig } from "~/components/ui/page-header/types";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDropdown } from "~/components/operator/status-dropdown";
import { OperatorCommentAccordion } from "~/components/operator/operator-comment-accordion";
import { useSession } from "next-auth/react";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorSuggestionsService } from "~/lib/operator/suggestions-api";
import { OPERATOR_STATUS_LABELS } from "~/lib/operator/suggestions-helpers";
import type {
  OperatorSuggestion,
  OperatorSuggestionStatus,
} from "~/lib/operator/suggestions-helpers";
import { getRelativeTime, getInitials } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";
import { operatorPath } from "~/lib/operator-url";

const logger = createLogger({ component: "OperatorSuggestionsPage" });

export default function OperatorSuggestionsPage() {
  const { status } = useSession();
  useSetBreadcrumb({ pageTitle: "Feedback" });
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [visibilityFilter, setVisibilityFilter] = useState("all");
  const [schoolFilter, setSchoolFilter] = useState("all");
  const [sourceFilter, setSourceFilter] = useState("all");
  const [statusUpdating, setStatusUpdating] = useState<string | null>(null);
  const [openDropdownId, setOpenDropdownId] = useState<string | null>(null);

  const {
    data: suggestions,
    isLoading,
    mutate,
  } = useSWR(
    status === "authenticated" ? "operator-suggestions" : null,
    () => operatorSuggestionsService.fetchAll(),
    {
      refreshInterval: 30000, // Refresh every 30 seconds to catch new posts
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
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

  const schoolOptions = useMemo(() => {
    if (!suggestions) return [];
    const schools = new Map<string, string>();
    for (const s of suggestions) {
      if (s.schoolId && s.schoolName) {
        schools.set(s.schoolId, s.schoolName);
      }
    }
    return Array.from(schools.entries())
      .map(([id, name]) => ({ value: id, label: name }))
      .sort((a, b) => a.label.localeCompare(b.label, "de"));
  }, [suggestions]);

  const filteredSuggestions = useMemo(() => {
    if (!suggestions) return [];
    let result = suggestions;
    if (statusFilter !== "all") {
      result = result.filter((s) => s.status === statusFilter);
    }
    if (visibilityFilter === "visible") {
      result = result.filter((s) => !s.isHidden);
    } else if (visibilityFilter === "hidden") {
      result = result.filter((s) => s.isHidden);
    }
    if (schoolFilter !== "all") {
      result = result.filter((s) => s.schoolId === schoolFilter);
    }
    if (sourceFilter !== "all") {
      result = result.filter((s) => s.authorType === sourceFilter);
    }
    if (searchTerm.trim()) {
      const term = searchTerm.toLowerCase();
      result = result.filter(
        (s) =>
          s.title.toLowerCase().includes(term) ||
          s.description.toLowerCase().includes(term) ||
          s.authorName.toLowerCase().includes(term) ||
          s.schoolName.toLowerCase().includes(term),
      );
    }
    return result;
  }, [
    suggestions,
    searchTerm,
    statusFilter,
    visibilityFilter,
    schoolFilter,
    sourceFilter,
  ]);

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
        logger.error("suggestion_status_update_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
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
    {
      id: "visibility",
      label: "Sichtbarkeit",
      type: "dropdown",
      value: visibilityFilter,
      onChange: (value) => setVisibilityFilter(value as string),
      options: [
        { value: "all", label: "Alle" },
        { value: "visible", label: "Sichtbar" },
        { value: "hidden", label: "Ausgeblendet" },
      ],
    },
    {
      id: "school",
      label: "Schule",
      type: "dropdown",
      value: schoolFilter,
      onChange: (value) => setSchoolFilter(value as string),
      options: [{ value: "all", label: "Alle Schulen" }, ...schoolOptions],
    },
    {
      id: "source",
      label: "Quelle",
      type: "dropdown",
      value: sourceFilter,
      onChange: (value) => setSourceFilter(value as string),
      options: [
        { value: "all", label: "Alle Quellen" },
        { value: "staff", label: "Personal" },
        { value: "parent", label: "Eltern" },
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
                  <OperatorSuggestionCard
                    suggestion={suggestion}
                    statusUpdating={statusUpdating === suggestion.id}
                    onStatusChange={(newStatus) =>
                      void handleStatusChange(suggestion.id, newStatus)
                    }
                    onDropdownOpenChange={(open) =>
                      setOpenDropdownId(open ? suggestion.id : null)
                    }
                  />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        </LayoutGroup>
      )}
    </div>
  );
}

function OperatorSuggestionCard({
  suggestion,
  statusUpdating,
  onStatusChange,
  onDropdownOpenChange,
}: {
  readonly suggestion: OperatorSuggestion;
  readonly statusUpdating: boolean;
  readonly onStatusChange: (status: OperatorSuggestionStatus) => void;
  readonly onDropdownOpenChange: (open: boolean) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [isClamped, setIsClamped] = useState(false);
  const descRef = useRef<HTMLParagraphElement>(null);

  useEffect(() => {
    const el = descRef.current;
    if (el) {
      setIsClamped(el.scrollHeight > el.clientHeight);
    }
  }, [suggestion.description]);

  return (
    <div
      className={`overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-sm backdrop-blur-md ${suggestion.isHidden ? "opacity-60" : ""}`}
    >
      <div className="p-5">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <h3 className="flex min-w-0 items-center gap-2 text-base font-semibold wrap-anywhere text-gray-900">
            <Link
              href={operatorPath(`/operator/suggestions/${suggestion.id}`)}
              className="hover:underline"
            >
              {suggestion.title}
            </Link>
            {suggestion.isNew && (
              <span className="bg-moto-blue shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold text-white">
                Neu
              </span>
            )}
            {suggestion.isHidden && (
              <span className="bg-moto-amber-soft text-moto-amber-strong shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold">
                Ausgeblendet
              </span>
            )}
            {suggestion.authorType === "parent" && (
              <span className="bg-moto-magenta/10 text-moto-magenta shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold">
                Eltern
              </span>
            )}
          </h3>
          <div className="flex shrink-0 items-center gap-2">
            <div className="flex items-center gap-2">
              <span className="text-moto-green flex items-center gap-1">
                <ThumbsUp className="h-4 w-4" fill="currentColor" />
                <span className="text-xs font-bold">{suggestion.upvotes}</span>
              </span>
              <span className="flex items-center gap-1 text-red-500">
                <ThumbsDown className="h-4 w-4" fill="currentColor" />
                <span className="text-xs font-bold">
                  {suggestion.downvotes}
                </span>
              </span>
            </div>
            <StatusDropdown
              value={suggestion.status}
              onChange={onStatusChange}
              disabled={statusUpdating}
              onOpenChange={onDropdownOpenChange}
            />
          </div>
        </div>
        <p
          ref={descRef}
          className={`mt-1 text-sm text-gray-600 ${expanded ? "" : "line-clamp-2"}`}
        >
          {suggestion.description}
        </p>
        {(isClamped || expanded) && (
          <button
            type="button"
            onClick={() => setExpanded((prev) => !prev)}
            className="mt-1 text-xs font-medium text-gray-500 transition-colors hover:text-gray-700"
          >
            {expanded ? "Weniger anzeigen" : "Mehr anzeigen"}
          </button>
        )}
        <div className="mt-3 flex items-center gap-2 text-xs text-gray-500">
          <span className="bg-moto-blue/15 text-moto-blue-hover flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-medium">
            {getInitials(suggestion.authorName)}
          </span>
          <span>{suggestion.authorName}</span>
          <span>·</span>
          <span>{getRelativeTime(suggestion.createdAt)}</span>
          {suggestion.schoolName && (
            <>
              <span>·</span>
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-600">
                {suggestion.schoolName}
              </span>
            </>
          )}
        </div>
      </div>
      <OperatorCommentAccordion
        postId={suggestion.id}
        commentCount={suggestion.commentCount}
        unreadCount={suggestion.unreadCount}
        isNew={suggestion.isNew}
      />
    </div>
  );
}

function SuggestionSkeletons() {
  return (
    <div className="mt-4 space-y-4">
      {Array.from({ length: 3 }, (_, i) => (
        <div
          key={i}
          className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-sm"
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
