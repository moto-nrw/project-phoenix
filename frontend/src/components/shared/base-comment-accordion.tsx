"use client";

import { useState, useCallback, useRef } from "react";
import { ArrowUp, ChevronDown, Trash2 } from "lucide-react";
import { ConfirmationModal } from "~/components/ui/modal";
import { getRelativeTime, getInitial } from "~/lib/format-utils";

export interface BaseComment {
  id: string;
  content: string;
  authorName: string;
  authorType: "operator" | "user";
  createdAt: string;
}

interface CommentAccordionConfig {
  loadComments: (postId: string) => Promise<BaseComment[]>;
  createComment: (postId: string, content: string) => Promise<void>;
  deleteComment: (postId: string, commentId: string) => Promise<void>;
  onOpen?: (postId: string) => void;
  onAfterCreate?: (
    postId: string,
    reloadComments: () => Promise<void>,
  ) => Promise<void>;
  canDeleteComment?: (comment: BaseComment) => boolean;
}

interface BaseCommentAccordionProps extends CommentAccordionConfig {
  readonly postId: string;
  readonly commentCount?: number;
  readonly unreadCount?: number;
}

export function BaseCommentAccordion({
  postId,
  commentCount,
  unreadCount,
  loadComments: loadCommentsFn,
  createComment: createCommentFn,
  deleteComment: deleteCommentFn,
  onOpen,
  onAfterCreate,
  canDeleteComment,
}: BaseCommentAccordionProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [comments, setComments] = useState<BaseComment[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [newComment, setNewComment] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleteCommentId, setDeleteCommentId] = useState<string | null>(null);
  const [isDeletingComment, setIsDeletingComment] = useState(false);
  const loadedRef = useRef(false);

  const reloadComments = useCallback(
    async (silent = false) => {
      if (!silent) setIsLoading(true);
      setError(null);
      try {
        const data = await loadCommentsFn(postId);
        setComments(data);
        loadedRef.current = true;
      } catch {
        setError("Kommentare konnten nicht geladen werden.");
      } finally {
        if (!silent) setIsLoading(false);
      }
    },
    [postId, loadCommentsFn],
  );

  const handleToggle = useCallback(() => {
    const opening = !isOpen;
    setIsOpen(opening);
    if (opening) {
      void reloadComments();
      onOpen?.(postId);
    }
  }, [isOpen, reloadComments, onOpen, postId]);

  const handleSubmit = useCallback(
    async (e: React.SyntheticEvent) => {
      e.preventDefault();
      if (!newComment.trim()) return;
      setIsSubmitting(true);
      setError(null);
      try {
        await createCommentFn(postId, newComment.trim());
        setNewComment("");
        if (onAfterCreate) {
          await onAfterCreate(postId, () => reloadComments(true));
        } else {
          await reloadComments(true);
        }
      } catch {
        setError("Kommentar konnte nicht gesendet werden.");
      } finally {
        setIsSubmitting(false);
      }
    },
    [postId, newComment, reloadComments, createCommentFn, onAfterCreate],
  );

  const handleDelete = useCallback(async () => {
    if (!deleteCommentId) return;
    setIsDeletingComment(true);
    setError(null);
    try {
      await deleteCommentFn(postId, deleteCommentId);
      setComments((prev) => prev.filter((c) => c.id !== deleteCommentId));
      setDeleteCommentId(null);
    } catch {
      setError("Kommentar konnte nicht gelöscht werden.");
    } finally {
      setIsDeletingComment(false);
    }
  }, [postId, deleteCommentId, deleteCommentFn]);

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
          <span className="flex items-center gap-1.5 font-medium">
            Kommentare{displayCount > 0 ? ` (${displayCount})` : ""}
            {(unreadCount ?? 0) > 0 && (
              <span className="rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold text-white">
                {unreadCount} neu
              </span>
            )}
          </span>
          <ChevronDown
            className={`h-4 w-4 transition-transform duration-200 ${isOpen ? "rotate-180" : ""}`}
          />
        </button>

        {/* Accordion body */}
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
                <div className="mb-3 space-y-0 divide-y divide-gray-100">
                  {comments.map((comment) => {
                    const isOperator = comment.authorType === "operator";
                    const showDelete = canDeleteComment
                      ? canDeleteComment(comment)
                      : true;

                    return (
                      <div
                        key={comment.id}
                        className="flex gap-2.5 py-2.5 first:pt-0 last:pb-0"
                      >
                        {/* Initials avatar */}
                        <div
                          className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-medium ${
                            isOperator
                              ? "bg-blue-100 text-blue-600"
                              : "bg-gray-100 text-gray-500"
                          }`}
                        >
                          {getInitial(comment.authorName)}
                        </div>

                        {/* Content */}
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center justify-between">
                            <div className="flex items-baseline gap-1.5">
                              <span className="text-sm font-medium text-gray-900">
                                {comment.authorName}
                              </span>
                              <span
                                className={`text-xs ${isOperator ? "text-blue-600" : "text-gray-400"}`}
                              >
                                {isOperator ? "moto Team" : "OGS Team"}
                              </span>
                              <time
                                dateTime={comment.createdAt}
                                title={new Date(
                                  comment.createdAt,
                                ).toLocaleString("de-DE")}
                                className="text-xs text-gray-400"
                              >
                                · {getRelativeTime(comment.createdAt)}
                              </time>
                            </div>
                            {showDelete && (
                              <button
                                type="button"
                                onClick={() => setDeleteCommentId(comment.id)}
                                className="rounded p-0.5 text-gray-300 transition-colors hover:bg-gray-100 hover:text-red-500"
                                aria-label="Kommentar löschen"
                              >
                                <Trash2 className="h-3 w-3" />
                              </button>
                            )}
                          </div>
                          <p className="mt-0.5 text-xs leading-relaxed whitespace-pre-wrap text-gray-600">
                            {comment.content}
                          </p>
                        </div>
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
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && !e.shiftKey) {
                      e.preventDefault();
                      if (newComment.trim() && !isSubmitting) {
                        void handleSubmit(e);
                      }
                    }
                  }}
                  placeholder="Kommentar schreiben..."
                  rows={1}
                  className="flex-1 resize-none rounded-lg border border-gray-200 px-3 py-2 text-xs transition-colors focus:border-gray-300 focus:ring-0 focus:outline-none"
                />
                <button
                  type="submit"
                  disabled={isSubmitting || !newComment.trim()}
                  className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gray-900 text-white transition-colors hover:bg-gray-700 disabled:opacity-30"
                  aria-label="Senden"
                >
                  <ArrowUp className="h-4 w-4" />
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
