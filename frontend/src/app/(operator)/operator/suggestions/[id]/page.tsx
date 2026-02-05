"use client";

import { useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import useSWR from "swr";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorSuggestionsService } from "~/lib/operator/suggestions-api";
import {
  OPERATOR_STATUS_LABELS,
  OPERATOR_STATUS_STYLES,
} from "~/lib/operator/suggestions-helpers";
import type { OperatorSuggestionStatus } from "~/lib/operator/suggestions-helpers";
import { ConfirmationModal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";

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

export default function OperatorSuggestionDetailPage() {
  const params = useParams();
  const router = useRouter();
  const { isAuthenticated } = useOperatorAuth();
  const id = params.id as string;
  useSetBreadcrumb({ pageTitle: "Vorschlag Details" });

  const [commentText, setCommentText] = useState("");
  const [isInternal, setIsInternal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [statusUpdating, setStatusUpdating] = useState(false);
  const [deleteCommentId, setDeleteCommentId] = useState<string | null>(null);
  const [isDeletingComment, setIsDeletingComment] = useState(false);

  const {
    data: suggestion,
    isLoading,
    mutate,
  } = useSWR(isAuthenticated && id ? `operator-suggestion-${id}` : null, () =>
    operatorSuggestionsService.fetchById(id),
  );

  const handleStatusChange = useCallback(
    async (newStatus: OperatorSuggestionStatus) => {
      if (!suggestion) return;
      setStatusUpdating(true);
      try {
        const updated = await operatorSuggestionsService.updateStatus(
          suggestion.id,
          newStatus,
        );
        await mutate(updated, { revalidate: false });
      } catch (error) {
        console.error("Failed to update status:", error);
      } finally {
        setStatusUpdating(false);
      }
    },
    [suggestion, mutate],
  );

  const handleAddComment = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!suggestion || !commentText.trim()) return;
      setIsSubmitting(true);
      try {
        await operatorSuggestionsService.addComment(
          suggestion.id,
          commentText.trim(),
          isInternal,
        );
        setCommentText("");
        setIsInternal(false);
        await mutate();
      } catch (error) {
        console.error("Failed to add comment:", error);
      } finally {
        setIsSubmitting(false);
      }
    },
    [suggestion, commentText, isInternal, mutate],
  );

  const handleDeleteComment = useCallback(async () => {
    if (!suggestion || !deleteCommentId) return;
    setIsDeletingComment(true);
    try {
      await operatorSuggestionsService.deleteComment(
        suggestion.id,
        deleteCommentId,
      );
      setDeleteCommentId(null);
      await mutate();
    } catch (error) {
      console.error("Failed to delete comment:", error);
    } finally {
      setIsDeletingComment(false);
    }
  }, [suggestion, deleteCommentId, mutate]);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-32 rounded" />
        <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
          <Skeleton className="mb-4 h-6 w-3/4 rounded" />
          <Skeleton className="mb-2 h-4 w-full rounded" />
          <Skeleton className="mb-2 h-4 w-full rounded" />
          <Skeleton className="h-4 w-2/3 rounded" />
        </div>
      </div>
    );
  }

  if (!suggestion) {
    return (
      <div className="py-12 text-center">
        <p className="text-lg font-medium text-gray-900">
          Vorschlag nicht gefunden
        </p>
        <button
          type="button"
          onClick={() => router.push("/operator/suggestions")}
          className="mt-4 text-sm text-blue-600 hover:underline"
        >
          Zurück zur Übersicht
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Back button */}
      <button
        type="button"
        onClick={() => router.push("/operator/suggestions")}
        className="flex items-center gap-1 text-sm text-gray-500 transition-colors hover:text-gray-700"
      >
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M15 19l-7-7 7-7"
          />
        </svg>
        Zurück
      </button>

      {/* Main card */}
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        {/* Status dropdown (prominent) */}
        <div className="mb-4 flex items-center justify-between">
          <select
            value={suggestion.status}
            onChange={(e) =>
              void handleStatusChange(
                e.target.value as OperatorSuggestionStatus,
              )
            }
            disabled={statusUpdating}
            className={`rounded-full border-0 px-3 py-1 text-sm font-medium ${OPERATOR_STATUS_STYLES[suggestion.status]} cursor-pointer focus:ring-2 focus:ring-blue-500 focus:outline-none disabled:opacity-50`}
          >
            {Object.entries(OPERATOR_STATUS_LABELS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
          <span className="text-sm text-gray-500">
            {suggestion.score} Stimmen ({suggestion.upvotes} /{" "}
            {suggestion.downvotes})
          </span>
        </div>

        {/* Title & description */}
        <h1 className="mb-2 text-xl font-bold text-gray-900">
          {suggestion.title}
        </h1>
        <p className="mb-4 whitespace-pre-wrap text-gray-600">
          {suggestion.description}
        </p>

        {/* Meta */}
        <div className="flex items-center gap-2 text-xs text-gray-500">
          <span>{suggestion.authorName}</span>
          <span>·</span>
          <span>{getRelativeTime(suggestion.createdAt)}</span>
        </div>
      </div>

      {/* Comments section */}
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <h2 className="mb-4 text-lg font-semibold text-gray-900">
          Kommentare ({suggestion.operatorComments.length})
        </h2>

        {/* Comment list */}
        {suggestion.operatorComments.length > 0 && (
          <div className="mb-6 space-y-3">
            {suggestion.operatorComments.map((comment) => (
              <div
                key={comment.id}
                className={`rounded-xl border p-4 ${
                  comment.isInternal
                    ? "border-yellow-200 bg-yellow-50"
                    : "border-blue-200 bg-blue-50"
                }`}
              >
                <div className="mb-1 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-900">
                      {comment.operatorName}
                    </span>
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                        comment.isInternal
                          ? "bg-yellow-200 text-yellow-800"
                          : "bg-blue-200 text-blue-800"
                      }`}
                    >
                      {comment.isInternal ? "Intern" : "Öffentlich"}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-500">
                      {getRelativeTime(comment.createdAt)}
                    </span>
                    <button
                      type="button"
                      onClick={() => setDeleteCommentId(comment.id)}
                      className="rounded p-1 text-gray-400 transition-colors hover:bg-gray-200 hover:text-red-500"
                      aria-label="Kommentar löschen"
                    >
                      <svg
                        className="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        strokeWidth={2}
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
                <p className="text-sm whitespace-pre-wrap text-gray-700">
                  {comment.content}
                </p>
              </div>
            ))}
          </div>
        )}

        {/* Add comment form */}
        <form onSubmit={(e) => void handleAddComment(e)} className="space-y-3">
          <textarea
            value={commentText}
            onChange={(e) => setCommentText(e.target.value)}
            placeholder="Kommentar schreiben..."
            rows={3}
            className="w-full rounded-xl border border-gray-200 p-3 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
          />
          <div className="flex items-center justify-between">
            <label className="flex items-center gap-2 text-sm text-gray-600">
              <input
                type="checkbox"
                checked={isInternal}
                onChange={(e) => setIsInternal(e.target.checked)}
                className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              Interne Notiz
            </label>
            <button
              type="submit"
              disabled={isSubmitting || !commentText.trim()}
              className="rounded-xl bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isSubmitting ? "Wird gesendet..." : "Senden"}
            </button>
          </div>
        </form>
      </div>

      {/* Delete comment confirmation */}
      <ConfirmationModal
        isOpen={!!deleteCommentId}
        onClose={() => setDeleteCommentId(null)}
        onConfirm={() => {
          void handleDeleteComment();
        }}
        title="Kommentar löschen?"
        confirmText="Löschen"
        confirmButtonClass="bg-red-500 hover:bg-red-600"
        isConfirmLoading={isDeletingComment}
      >
        <p className="text-sm text-gray-600">
          Dieser Kommentar wird unwiderruflich gelöscht.
        </p>
      </ConfirmationModal>
    </div>
  );
}
