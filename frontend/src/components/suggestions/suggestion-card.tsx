"use client";

import { useState, useRef, useEffect } from "react";
import type { Suggestion } from "~/lib/suggestions-helpers";
import type { SuggestionsBoardApi } from "~/lib/suggestions-board-api";
import { staffBoardApi } from "~/lib/suggestions-board-api";
import { STATUS_LABELS, STATUS_STYLES } from "~/lib/suggestions-helpers";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { CommentAccordion } from "./comment-accordion";
import { VoteButtons } from "./vote-buttons";
import { getRelativeTime, getInitials } from "~/lib/format-utils";

/** Stable default so the prop does not break referential equality per render. */
const DEFAULT_MENU_LABELS = {
  edit: "Bearbeiten",
  delete: "Löschen",
  actions: "Aktionen",
} as const;

interface SuggestionCardProps {
  readonly suggestion: Suggestion;
  readonly currentAccountId: string;
  readonly onEdit: (s: Suggestion) => void;
  readonly onDelete: (s: Suggestion) => void;
  readonly onVoteChange: (updated: Suggestion) => void;
  /** Which board this card belongs to. Defaults to the staff board. */
  readonly api?: SuggestionsBoardApi;
  /** Enable analytics only from the authenticated tenant board. */
  readonly analyticsEnabled?: boolean;
  /** Unread-refresh event name forwarded to the comment accordion. */
  readonly unreadRefreshEvent?: string;
  /**
   * Status wording. Defaults to the German staff labels; the parents portal is
   * localized and passes translated ones.
   */
  readonly statusLabels?: Record<Suggestion["status"], string>;
  /** Action wording, same reason as statusLabels. */
  readonly menuLabels?: { edit: string; delete: string; actions: string };
}

export function SuggestionCard({
  suggestion,
  currentAccountId,
  onEdit,
  onDelete,
  onVoteChange,
  api = staffBoardApi,
  analyticsEnabled = false,
  unreadRefreshEvent,
  statusLabels = STATUS_LABELS,
  menuLabels = DEFAULT_MENU_LABELS,
}: SuggestionCardProps) {
  const [expanded, setExpanded] = useState(false);
  const [isClamped, setIsClamped] = useState(false);
  const descRef = useRef<HTMLParagraphElement>(null);
  // The parent feedback board ships no author id, so the backend decides
  // ownership there; the staff board keeps comparing ids.
  const isOwner = suggestion.isOwn ?? suggestion.authorId === currentAccountId;

  useEffect(() => {
    const el = descRef.current;
    if (el) {
      setIsClamped(el.scrollHeight > el.clientHeight);
    }
  }, [suggestion.description]);

  return (
    <div className="overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-sm backdrop-blur-md transition-all duration-150 md:hover:-translate-y-0.5 md:hover:border-blue-200/50 md:hover:shadow-sm">
      <div className="flex flex-col gap-3 p-5 md:flex-row md:gap-4">
        {/* Vote column - hidden on mobile, shown on desktop */}
        <div className="hidden md:flex md:items-start md:pt-1">
          <VoteButtons
            suggestion={suggestion}
            onVoteChange={onVoteChange}
            api={api}
            analyticsEnabled={analyticsEnabled}
          />
        </div>

        {/* Content */}
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <h3 className="min-w-0 text-base font-semibold wrap-anywhere text-gray-900">
              {suggestion.title}
            </h3>
            <div className="flex shrink-0 items-center gap-2">
              <span
                className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${STATUS_STYLES[suggestion.status]}`}
              >
                {statusLabels[suggestion.status]}
              </span>
              {isOwner && (
                <OverflowMenu
                  ariaLabel={menuLabels.actions}
                  items={[
                    {
                      label: menuLabels.edit,
                      onClick: () => onEdit(suggestion),
                    },
                    {
                      label: menuLabels.delete,
                      destructive: true,
                      onClick: () => onDelete(suggestion),
                    },
                  ]}
                />
              )}
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

          {/* Meta row + mobile vote */}
          <div className="mt-3 flex items-center justify-between">
            <div className="flex items-center gap-2 text-xs text-gray-500">
              <span className="flex h-5 w-5 items-center justify-center rounded-full bg-blue-100 text-[10px] font-medium text-blue-700">
                {getInitials(suggestion.authorName)}
              </span>
              <span>{suggestion.authorName}</span>
              <span>·</span>
              <span>{getRelativeTime(suggestion.createdAt)}</span>
            </div>
            {/* Mobile vote buttons */}
            <div className="md:hidden">
              <VoteButtons
                suggestion={suggestion}
                onVoteChange={onVoteChange}
                api={api}
                analyticsEnabled={analyticsEnabled}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Comment accordion */}
      <CommentAccordion
        postId={suggestion.id}
        currentAccountId={currentAccountId}
        commentCount={suggestion.commentCount}
        unreadCount={suggestion.unreadCount}
        api={api}
        unreadRefreshEvent={unreadRefreshEvent}
      />
    </div>
  );
}
