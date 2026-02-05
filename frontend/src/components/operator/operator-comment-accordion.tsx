"use client";

import { useState, useCallback, useRef } from "react";
import { ChevronDown, Trash2 } from "lucide-react";
import { operatorSuggestionsService } from "~/lib/operator/suggestions-api";
import type { OperatorComment } from "~/lib/operator/suggestions-helpers";
import { ConfirmationModal } from "~/components/ui/modal";

function formatUnit(value: number, singular: string, plural: string): string {
  return `vor ${value} ${value === 1 ? singular : plural}`;
}

function getRelativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "gerade eben";
  if (minutes < 60) return formatUnit(minutes, "Minute", "Minuten");

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return formatUnit(hours, "Stunde", "Stunden");

  const days = Math.floor(hours / 24);
  if (days < 7) return formatUnit(days, "Tag", "Tagen");

  const weeks = Math.floor(days / 7);
  if (weeks < 5) return formatUnit(weeks, "Woche", "Wochen");

  const months = Math.floor(days / 30);
  if (months < 12) return formatUnit(months, "Monat", "Monaten");

  const years = Math.floor(days / 365);
  return formatUnit(years, "Jahr", "Jahren");
}

interface OperatorCommentAccordionProps {
  readonly postId: string;
  readonly commentCount?: number;
}

export function OperatorCommentAccordion({
  postId,
  commentCount,
}: OperatorCommentAccordionProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [comments, setComments] = useState<OperatorComment[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [newComment, setNewComment] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleteCommentId, setDeleteCommentId] = useState<string | null>(null);
  const [isDeletingComment, setIsDeletingComment] = useState(false);
  const loadedRef = useRef(false);

  const loadComments = useCallback(async () => {
    if (loadedRef.current) return;
    setIsLoading(true);
    setError(null);
    try {
      const suggestion = await operatorSuggestionsService.fetchById(postId);
      setComments(suggestion.operatorComments);
      loadedRef.current = true;
    } catch {
      setError("Kommentare konnten nicht geladen werden.");
    } finally {
      setIsLoading(false);
    }
  }, [postId]);

  const handleToggle = useCallback(() => {
    const opening = !isOpen;
    setIsOpen(opening);
    if (opening) {
      void loadComments();
    }
  }, [isOpen, loadComments]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!newComment.trim()) return;
      setIsSubmitting(true);
      setError(null);
      try {
        await operatorSuggestionsService.addComment(
          postId,
          newComment.trim(),
          false,
        );
        setNewComment("");
        loadedRef.current = false;
        await loadComments();
      } catch {
        setError("Kommentar konnte nicht gesendet werden.");
      } finally {
        setIsSubmitting(false);
      }
    },
    [postId, newComment, loadComments],
  );

  const handleDelete = useCallback(async () => {
    if (!deleteCommentId) return;
    setIsDeletingComment(true);
    setError(null);
    try {
      await operatorSuggestionsService.deleteComment(postId, deleteCommentId);
      setComments((prev) => prev.filter((c) => c.id !== deleteCommentId));
      setDeleteCommentId(null);
    } catch {
      setError("Kommentar konnte nicht gelöscht werden.");
    } finally {
      setIsDeletingComment(false);
    }
  }, [postId, deleteCommentId]);

  const displayCount = loadedRef.current
    ? comments.length
    : (commentCount ?? 0);

  return (
    <>
      <div
        className="border-t border-gray-100"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => e.stopPropagation()}
        role="presentation"
      >
        {/* Accordion header */}
        <button
          type="button"
          onClick={handleToggle}
          className="flex w-full items-center justify-between px-5 py-3 text-sm text-gray-600 transition-colors hover:text-gray-900"
        >
          <span className="font-medium">
            Kommentare{displayCount > 0 ? ` (${displayCount})` : ""}
          </span>
          <ChevronDown
            className={`h-4 w-4 transition-transform duration-200 ${isOpen ? "rotate-180" : ""}`}
          />
        </button>

        {/* Accordion body — CSS Grid for smooth animation */}
        <div
          className={`grid transition-[grid-template-rows] duration-200 ${isOpen ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}
        >
          <div className="overflow-hidden">
            <div className="px-5 pb-4">
              {/* Loading state */}
              {isLoading && (
                <p className="py-2 text-xs text-gray-400">Laden...</p>
              )}

              {/* Error */}
              {error && <p className="mb-2 text-xs text-red-500">{error}</p>}

              {/* Comment list */}
              {!isLoading && comments.length > 0 && (
                <div className="mb-3 space-y-2">
                  {comments.map((comment) => {
                    const isOperator = comment.authorType === "operator";
                    const borderClass = isOperator
                      ? "border-blue-100"
                      : "border-green-100";
                    const bgClass = isOperator
                      ? "bg-blue-50/50"
                      : "bg-green-50/50";
                    const badgeClass = isOperator
                      ? "bg-blue-100 text-blue-700"
                      : "bg-green-100 text-green-700";
                    const badgeText = isOperator ? "moto Team" : "OGS-Benutzer";

                    return (
                      <div
                        key={comment.id}
                        className={`rounded-lg border p-3 ${borderClass} ${bgClass}`}
                      >
                        <div className="mb-1 flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <span className="text-xs font-medium text-gray-900">
                              {comment.authorName}
                            </span>
                            <span
                              className={`rounded-full px-1.5 py-0.5 text-[10px] font-medium ${badgeClass}`}
                            >
                              {badgeText}
                            </span>
                            <span className="text-[10px] text-gray-400">
                              {getRelativeTime(comment.createdAt)}
                            </span>
                          </div>
                          <button
                            type="button"
                            onClick={() => setDeleteCommentId(comment.id)}
                            className="rounded p-0.5 text-gray-300 transition-colors hover:bg-gray-100 hover:text-red-500"
                            aria-label="Kommentar löschen"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </div>
                        <p className="text-xs whitespace-pre-wrap text-gray-700">
                          {comment.content}
                        </p>
                      </div>
                    );
                  })}
                </div>
              )}

              {/* Empty state */}
              {!isLoading && loadedRef.current && comments.length === 0 && (
                <p className="mb-3 text-xs text-gray-400">
                  Noch keine Kommentare.
                </p>
              )}

              {/* Comment input */}
              <form
                onSubmit={(e) => void handleSubmit(e)}
                className="flex items-end gap-2"
              >
                <textarea
                  value={newComment}
                  onChange={(e) => setNewComment(e.target.value)}
                  placeholder="Kommentar schreiben..."
                  rows={1}
                  className="flex-1 resize-none rounded-lg border border-gray-200 px-3 py-2 text-xs transition-all duration-200 focus:border-gray-300 focus:ring-0 focus:outline-none"
                />
                <button
                  type="submit"
                  disabled={isSubmitting || !newComment.trim()}
                  className="shrink-0 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {isSubmitting ? "Senden..." : "Senden"}
                </button>
              </form>
            </div>
          </div>
        </div>
      </div>

      {/* Delete comment confirmation */}
      <ConfirmationModal
        isOpen={!!deleteCommentId}
        onClose={() => setDeleteCommentId(null)}
        onConfirm={() => {
          void handleDelete();
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
    </>
  );
}
