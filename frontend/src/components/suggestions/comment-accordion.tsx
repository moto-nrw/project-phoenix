"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { SuggestionsBoardApi } from "~/lib/suggestions-board-api";
import { staffBoardApi } from "~/lib/suggestions-board-api";
import {
  BaseCommentAccordion,
  type BaseComment,
} from "~/components/shared/base-comment-accordion";

interface CommentAccordionProps {
  readonly postId: string;
  readonly currentAccountId: string;
  readonly commentCount?: number;
  readonly unreadCount?: number;
  /** Which board the thread belongs to. Defaults to the staff board. */
  readonly api?: SuggestionsBoardApi;
  /**
   * Event fired after the unread state changed, so the matching sidebar badge
   * can refetch. The parent feedback board uses its own event name.
   */
  readonly unreadRefreshEvent?: string;
}

export function CommentAccordion({
  postId,
  currentAccountId,
  commentCount,
  unreadCount,
  api = staffBoardApi,
  unreadRefreshEvent = "suggestions-unread-refresh",
}: CommentAccordionProps) {
  const [localUnreadCount, setLocalUnreadCount] = useState(unreadCount ?? 0);
  const markReadTriggered = useRef(false);

  // Sync local state when props change (e.g. if parent adds polling later)
  useEffect(() => {
    const newCount = unreadCount ?? 0;
    setLocalUnreadCount(newCount);
    if (newCount > 0) {
      markReadTriggered.current = false;
    }
  }, [unreadCount]);

  const handleOpen = useCallback(
    (pid: string) => {
      if (localUnreadCount > 0 && !markReadTriggered.current) {
        markReadTriggered.current = true;
        void api.markCommentsRead(pid).then(() => {
          setLocalUnreadCount(0);
          window.dispatchEvent(new CustomEvent(unreadRefreshEvent));
        });
      }
    },
    [localUnreadCount, api, unreadRefreshEvent],
  );

  const handleAfterCreate = useCallback(
    async (pid: string, reloadComments: () => Promise<void>) => {
      await reloadComments();
      void api.markCommentsRead(pid).then(() => {
        window.dispatchEvent(new CustomEvent(unreadRefreshEvent));
      });
    },
    [api, unreadRefreshEvent],
  );

  const canDeleteComment = useCallback(
    (comment: BaseComment) => {
      // The parent board is pseudonymous: it ships no author id, so the
      // backend marks the caller's own comments instead.
      if (comment.authorType === "parent") {
        return comment.isOwn === true;
      }
      return (
        comment.authorType === "user" && comment.authorId === currentAccountId
      );
    },
    [currentAccountId],
  );

  return (
    <BaseCommentAccordion
      postId={postId}
      commentCount={commentCount}
      unreadCount={localUnreadCount}
      loadComments={api.fetchComments}
      createComment={api.createComment}
      deleteComment={api.deleteComment}
      onOpen={handleOpen}
      onAfterCreate={handleAfterCreate}
      canDeleteComment={canDeleteComment}
    />
  );
}
