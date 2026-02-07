"use client";

import { useState, useRef, useEffect } from "react";
import type { Suggestion } from "~/lib/suggestions-helpers";
import { STATUS_LABELS, STATUS_STYLES } from "~/lib/suggestions-helpers";
import { CommentAccordion } from "./comment-accordion";
import { VoteButtons } from "./vote-buttons";
import { getRelativeTime, getInitials } from "~/lib/format-utils";

interface SuggestionCardProps {
  readonly suggestion: Suggestion;
  readonly currentAccountId: string;
  readonly onEdit: (s: Suggestion) => void;
  readonly onDelete: (s: Suggestion) => void;
  readonly onVoteChange: (updated: Suggestion) => void;
}

export function SuggestionCard({
  suggestion,
  currentAccountId,
  onEdit,
  onDelete,
  onVoteChange,
}: SuggestionCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [isClamped, setIsClamped] = useState(false);
  const descRef = useRef<HTMLParagraphElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const isOwner = suggestion.authorId === currentAccountId;

  useEffect(() => {
    const el = descRef.current;
    if (el) {
      setIsClamped(el.scrollHeight > el.clientHeight);
    }
  }, [suggestion.description]);

  useEffect(() => {
    if (!menuOpen) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [menuOpen]);

  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150 md:hover:-translate-y-0.5 md:hover:border-blue-200/50 md:hover:shadow-[0_12px_40px_rgb(0,0,0,0.18)]">
      <div className="flex flex-col gap-3 p-5 md:flex-row md:gap-4">
        {/* Vote column - hidden on mobile, shown on desktop */}
        <div className="hidden md:flex md:items-start md:pt-1">
          <VoteButtons suggestion={suggestion} onVoteChange={onVoteChange} />
        </div>

        {/* Content */}
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <h3 className="text-base font-semibold text-gray-900">
              {suggestion.title}
            </h3>
            <div className="flex shrink-0 items-center gap-2">
              <span
                className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${STATUS_STYLES[suggestion.status]}`}
              >
                {STATUS_LABELS[suggestion.status]}
              </span>
              {isOwner && (
                <div className="relative" ref={menuRef}>
                  <button
                    type="button"
                    onClick={() => setMenuOpen(!menuOpen)}
                    className="rounded-lg p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
                    aria-label="Aktionen"
                  >
                    <svg
                      className="h-5 w-5"
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" />
                    </svg>
                  </button>
                  {menuOpen && (
                    <div className="absolute right-0 z-10 mt-1 w-36 rounded-lg border border-gray-200 bg-white p-1 shadow-lg">
                      <button
                        type="button"
                        onClick={() => {
                          setMenuOpen(false);
                          onEdit(suggestion);
                        }}
                        className="flex w-full items-center rounded-md px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-100"
                      >
                        Bearbeiten
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setMenuOpen(false);
                          onDelete(suggestion);
                        }}
                        className="flex w-full items-center rounded-md px-3 py-1.5 text-sm text-red-600 hover:bg-red-50"
                      >
                        Löschen
                      </button>
                    </div>
                  )}
                </div>
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
      />
    </div>
  );
}
