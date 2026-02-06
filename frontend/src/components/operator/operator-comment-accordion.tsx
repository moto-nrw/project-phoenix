"use client";

import { useCallback, useRef, useState } from "react";
import { operatorSuggestionsService } from "~/lib/operator/suggestions-api";
import { BaseCommentAccordion } from "~/components/shared/base-comment-accordion";

interface OperatorCommentAccordionProps {
  readonly postId: string;
  readonly commentCount?: number;
  readonly unreadCount?: number;
  readonly isNew?: boolean;
}

export function OperatorCommentAccordion({
  postId,
  commentCount,
  unreadCount,
  isNew,
}: OperatorCommentAccordionProps) {
  const [localUnreadCount, setLocalUnreadCount] = useState(unreadCount ?? 0);
  const [localIsNew, setLocalIsNew] = useState(isNew ?? false);
  const markReadTriggered = useRef(false);

  const loadComments = useCallback(async (pid: string) => {
    const suggestion = await operatorSuggestionsService.fetchById(pid);
    return suggestion.operatorComments;
  }, []);

  const createComment = useCallback(async (pid: string, content: string) => {
    await operatorSuggestionsService.addComment(pid, content, false);
  }, []);

  const deleteComment = useCallback(async (pid: string, commentId: string) => {
    await operatorSuggestionsService.deleteComment(pid, commentId);
  }, []);

  const handleOpen = useCallback(
    (pid: string) => {
      if (localIsNew) {
        void operatorSuggestionsService.markPostViewed(pid).then(() => {
          setLocalIsNew(false);
          window.dispatchEvent(
            new CustomEvent("operator-suggestions-unviewed-refresh"),
          );
        });
      }
      if (localUnreadCount > 0 && !markReadTriggered.current) {
        markReadTriggered.current = true;
        void operatorSuggestionsService.markCommentsRead(pid).then(() => {
          setLocalUnreadCount(0);
          window.dispatchEvent(
            new CustomEvent("operator-suggestions-unread-refresh"),
          );
        });
      }
    },
    [localIsNew, localUnreadCount],
  );

  const handleAfterCreate = useCallback(
    async (pid: string, reloadComments: () => Promise<void>) => {
      await operatorSuggestionsService.markCommentsRead(pid);
      setLocalUnreadCount(0);
      window.dispatchEvent(
        new CustomEvent("operator-suggestions-unread-refresh"),
      );
      await reloadComments();
    },
    [],
  );

  return (
    <BaseCommentAccordion
      postId={postId}
      commentCount={commentCount}
      unreadCount={localUnreadCount}
      loadComments={loadComments}
      createComment={createComment}
      deleteComment={deleteComment}
      onOpen={handleOpen}
      onAfterCreate={handleAfterCreate}
    />
  );
}
