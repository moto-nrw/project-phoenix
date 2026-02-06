"use client";

import { useState, useMemo, useCallback, Suspense } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { AnimatePresence, LayoutGroup, motion } from "framer-motion";
import {
  PageHeaderWithSearch,
  type FilterConfig,
} from "~/components/ui/page-header";
import { ConfirmationModal } from "~/components/ui/modal";
import { SuggestionCard } from "~/components/suggestions/suggestion-card";
import { Skeleton } from "~/components/ui/skeleton";
import { SuggestionForm } from "~/components/suggestions/suggestion-form";
import { useSWRAuth } from "~/lib/swr";
import { fetchSuggestions, deleteSuggestion } from "~/lib/suggestions-api";
import { useToast } from "~/contexts/ToastContext";
import { STATUS_LABELS } from "~/lib/suggestions-helpers";
import type { Suggestion, SortOption } from "~/lib/suggestions-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SuggestionsPage" });

function SuggestionsPageContent() {
  const router = useRouter();
  const { data: session } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });
  const { success: toastSuccess, error: toastError } = useToast();

  const [searchTerm, setSearchTerm] = useState("");
  const [sortBy, setSortBy] = useState<SortOption>("score");
  const [statusFilter, setStatusFilter] = useState("all");
  const [formOpen, setFormOpen] = useState(false);
  const [editSuggestion, setEditSuggestion] = useState<Suggestion | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Suggestion | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const {
    data: suggestions,
    isLoading,
    mutate,
  } = useSWRAuth<Suggestion[]>(`suggestions-${sortBy}`, () =>
    fetchSuggestions(sortBy),
  );

  const accountId = session?.user?.id ?? "";

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

  const handleVoteChange = useCallback(
    (updated: Suggestion) => {
      mutate(
        (current) => {
          if (!current) return current;
          const next = current.map((s) => (s.id === updated.id ? updated : s));
          // Re-sort client-side to match the current sort order
          if (sortBy === "score") {
            next.sort((a, b) => {
              const scoreDiff = b.score - a.score;
              if (scoreDiff !== 0) return scoreDiff;
              return (
                new Date(b.createdAt).getTime() -
                new Date(a.createdAt).getTime()
              );
            });
          }
          return next;
        },
        { revalidate: false },
      ).catch(() => undefined);
    },
    [mutate, sortBy],
  );

  const handleEdit = useCallback((s: Suggestion) => {
    setEditSuggestion(s);
    setFormOpen(true);
  }, []);

  const handleFormClose = useCallback(() => {
    setFormOpen(false);
    setEditSuggestion(null);
  }, []);

  const handleFormSuccess = useCallback(() => {
    mutate().catch(() => undefined);
  }, [mutate]);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    try {
      await deleteSuggestion(deleteTarget.id);
      toastSuccess("Beitrag wurde gelöscht.");
      setDeleteTarget(null);
      mutate().catch(() => undefined);
    } catch (err) {
      logger.error("delete_suggestion_failed", {
        error: err instanceof Error ? err.message : String(err),
        suggestion_id: deleteTarget.id,
      });
      toastError("Fehler beim Löschen des Beitrags.");
    } finally {
      setIsDeleting(false);
    }
  }, [deleteTarget, mutate, toastSuccess, toastError]);

  const filterConfigs: FilterConfig[] = [
    {
      id: "sort",
      label: "Sortierung",
      type: "buttons",
      value: sortBy,
      onChange: (value) => setSortBy(value as SortOption),
      options: [
        { value: "score", label: "Beliebteste" },
        { value: "newest", label: "Neueste" },
      ],
    },
    {
      id: "status",
      label: "Status",
      type: "dropdown",
      value: statusFilter,
      onChange: (value) => setStatusFilter(value as string),
      options: [
        { value: "all", label: "Alle Status" },
        ...Object.entries(STATUS_LABELS).map(([value, label]) => ({
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
        actionButton={
          suggestions && suggestions.length > 0 ? (
            <button
              type="button"
              onClick={() => setFormOpen(true)}
              className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
            >
              Neuer Beitrag
            </button>
          ) : undefined
        }
        mobileActionButton={
          suggestions && suggestions.length > 0 ? (
            <button
              type="button"
              onClick={() => setFormOpen(true)}
              className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
              aria-label="Neuer Beitrag"
            >
              <svg
                className="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 4v16m8-8H4"
                />
              </svg>
            </button>
          ) : undefined
        }
      />

      {isLoading && <SuggestionSkeletons />}
      {!isLoading && filteredSuggestions.length === 0 && (
        <EmptyState
          hasSearch={searchTerm.trim().length > 0}
          onCreateClick={() => setFormOpen(true)}
        />
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
                  <SuggestionCard
                    suggestion={suggestion}
                    currentAccountId={accountId}
                    onEdit={handleEdit}
                    onDelete={setDeleteTarget}
                    onVoteChange={handleVoteChange}
                  />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        </LayoutGroup>
      )}

      <SuggestionForm
        isOpen={formOpen}
        onClose={handleFormClose}
        onSuccess={handleFormSuccess}
        editSuggestion={editSuggestion}
      />

      <ConfirmationModal
        isOpen={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          handleDelete().catch(() => undefined);
        }}
        title="Beitrag löschen?"
        confirmText="Löschen"
        confirmButtonClass="bg-red-500 hover:bg-red-600"
        isConfirmLoading={isDeleting}
      >
        <p className="text-sm text-gray-600">
          Dieser Beitrag und alle Stimmen werden unwiderruflich gelöscht.
        </p>
      </ConfirmationModal>
    </div>
  );
}

function EmptyState({
  hasSearch,
  onCreateClick,
}: {
  readonly hasSearch: boolean;
  readonly onCreateClick: () => void;
}) {
  if (hasSearch) {
    return (
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
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        <p className="text-lg font-medium text-gray-900">
          Keine Ergebnisse gefunden
        </p>
        <p className="text-sm text-gray-500">
          Versuche einen anderen Suchbegriff.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-4 py-12 text-center">
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
      <p className="text-lg font-medium text-gray-900">Noch keine Beiträge</p>
      <p className="text-sm text-gray-500">
        Teile Ideen, melde Probleme oder schlage Verbesserungen vor.
      </p>
      <button
        type="button"
        onClick={onCreateClick}
        className="mt-2 rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        Neuer Beitrag
      </button>
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

export default function SuggestionsPage() {
  return (
    <Suspense
      fallback={
        <div className="-mt-1.5 w-full">
          <SuggestionSkeletons />
        </div>
      }
    >
      <SuggestionsPageContent />
    </Suspense>
  );
}
