"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  fetchComments,
  createComment,
  deleteComment,
  markCommentsRead,
} from "~/lib/suggestions-api";
import {
  BaseCommentAccordion,
  type BaseComment,
} from "~/components/shared/base-comment-accordion";

interface CommentAccordionProps {
  readonly postId: string;
  readonly currentAccountId: string;
  readonly commentCount?: number;
  readonly unreadCount?: number;
}

export function CommentAccordion({
  postId,
  currentAccountId,
  commentCount,
  unreadCount,
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
        void markCommentsRead(pid).then(() => {
          setLocalUnreadCount(0);
          window.dispatchEvent(new CustomEvent("suggestions-unread-refresh"));
        });
      }
    },
    [localUnreadCount],
  );

  const handleAfterCreate = useCallback(
    async (pid: string, reloadComments: () => Promise<void>) => {
      await reloadComments();
      void markCommentsRead(pid).then(() => {
        window.dispatchEvent(new CustomEvent("suggestions-unread-refresh"));
      });
    },
    [],
  );

  const canDeleteComment = useCallback(
    (comment: BaseComment) =>
      comment.authorType === "user" && comment.authorId === currentAccountId,
    [currentAccountId],
  );

  return (
    <BaseCommentAccordion
      postId={postId}
      commentCount={commentCount}
      unreadCount={localUnreadCount}
      loadComments={fetchComments}
      createComment={createComment}
      deleteComment={deleteComment}
      onOpen={handleOpen}
      onAfterCreate={handleAfterCreate}
      canDeleteComment={canDeleteComment}
    />
  );
}
