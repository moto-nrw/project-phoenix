"use client";

import { useState, useCallback } from "react";
import type { Suggestion } from "~/lib/suggestions-helpers";
import { voteSuggestion, removeVote } from "~/lib/suggestions-api";

interface VoteButtonsProps {
  readonly suggestion: Suggestion;
  readonly onVoteChange: (updated: Suggestion) => void;
}

export function VoteButtons({ suggestion, onVoteChange }: VoteButtonsProps) {
  const [optimistic, setOptimistic] = useState<{
    score: number;
    userVote: "up" | "down" | null;
  } | null>(null);

  const score = optimistic?.score ?? suggestion.score;
  const userVote = optimistic?.userVote ?? suggestion.userVote;

  const handleVote = useCallback(
    async (direction: "up" | "down") => {
      const isToggleOff = userVote === direction;

      // Optimistic update
      const prevScore = score;
      const prevVote = userVote;

      let newScore = suggestion.score;
      if (isToggleOff) {
        // Removing vote
        newScore = suggestion.score - (direction === "up" ? 1 : -1);
        setOptimistic({ score: newScore, userVote: null });
      } else {
        // Adding or changing vote
        const delta = direction === "up" ? 1 : -1;
        const prevDelta = prevVote === "up" ? 1 : prevVote === "down" ? -1 : 0;
        newScore = suggestion.score - prevDelta + delta;
        setOptimistic({ score: newScore, userVote: direction });
      }

      try {
        const updated = isToggleOff
          ? await removeVote(suggestion.id)
          : await voteSuggestion(suggestion.id, direction);
        setOptimistic(null);
        onVoteChange(updated);
      } catch {
        // Revert on error
        setOptimistic({ score: prevScore, userVote: prevVote });
      }
    },
    [suggestion, score, userVote, onVoteChange],
  );

  const upClasses =
    userVote === "up"
      ? "text-blue-600 hover:text-blue-700"
      : "text-gray-400 hover:text-gray-600";

  const downClasses =
    userVote === "down"
      ? "text-red-500 hover:text-red-600"
      : "text-gray-400 hover:text-gray-600";

  const scoreClasses =
    userVote === "up"
      ? "text-blue-600"
      : userVote === "down"
        ? "text-red-500"
        : "text-gray-700";

  return (
    <div className="flex items-center gap-1 md:w-12 md:flex-col md:gap-0">
      <button
        type="button"
        onClick={() => void handleVote("up")}
        aria-pressed={userVote === "up"}
        aria-label="Positiv bewerten"
        className={`rounded-lg p-1 transition-colors ${upClasses}`}
      >
        <svg
          className="h-5 w-5 md:h-6 md:w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M5 15l7-7 7 7"
          />
        </svg>
      </button>
      <span
        className={`min-w-[2ch] text-center text-base font-bold md:text-lg ${scoreClasses}`}
      >
        {score}
      </span>
      <button
        type="button"
        onClick={() => void handleVote("down")}
        aria-pressed={userVote === "down"}
        aria-label="Negativ bewerten"
        className={`rounded-lg p-1 transition-colors ${downClasses}`}
      >
        <svg
          className="h-5 w-5 md:h-6 md:w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>
    </div>
  );
}
