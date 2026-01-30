"use client";

import { useState, useCallback } from "react";
import { ThumbsUp, ThumbsDown } from "lucide-react";
import type { Suggestion } from "~/lib/suggestions-helpers";
import { voteSuggestion, removeVote } from "~/lib/suggestions-api";

interface VoteButtonsProps {
  readonly suggestion: Suggestion;
  readonly onVoteChange: (updated: Suggestion) => void;
}

export function VoteButtons({ suggestion, onVoteChange }: VoteButtonsProps) {
  const [optimistic, setOptimistic] = useState<{
    upvotes: number;
    downvotes: number;
    userVote: "up" | "down" | null;
  } | null>(null);

  const upvotes = optimistic?.upvotes ?? suggestion.upvotes;
  const downvotes = optimistic?.downvotes ?? suggestion.downvotes;
  const userVote = optimistic?.userVote ?? suggestion.userVote;

  const handleVote = useCallback(
    async (direction: "up" | "down") => {
      const prevUpvotes = upvotes;
      const prevDownvotes = downvotes;
      const prevVote = userVote;

      // Toggle off: clicking active vote removes it
      if (userVote === direction) {
        const newUpvotes =
          direction === "up" ? suggestion.upvotes - 1 : suggestion.upvotes;
        const newDownvotes =
          direction === "down"
            ? suggestion.downvotes - 1
            : suggestion.downvotes;
        setOptimistic({
          upvotes: newUpvotes,
          downvotes: newDownvotes,
          userVote: null,
        });

        try {
          const updated = await removeVote(suggestion.id);
          setOptimistic(null);
          onVoteChange(updated);
        } catch {
          setOptimistic({
            upvotes: prevUpvotes,
            downvotes: prevDownvotes,
            userVote: prevVote,
          });
        }
        return;
      }

      // New vote or changing direction
      let newUpvotes = suggestion.upvotes;
      let newDownvotes = suggestion.downvotes;

      // Remove previous vote count
      if (prevVote === "up") newUpvotes--;
      if (prevVote === "down") newDownvotes--;

      // Add new vote count
      if (direction === "up") newUpvotes++;
      if (direction === "down") newDownvotes++;

      setOptimistic({
        upvotes: newUpvotes,
        downvotes: newDownvotes,
        userVote: direction,
      });

      try {
        const updated = await voteSuggestion(suggestion.id, direction);
        setOptimistic(null);
        onVoteChange(updated);
      } catch {
        setOptimistic({
          upvotes: prevUpvotes,
          downvotes: prevDownvotes,
          userVote: prevVote,
        });
      }
    },
    [suggestion, upvotes, downvotes, userVote, onVoteChange],
  );

  const upClasses =
    userVote === "up"
      ? "text-[#83CD2D] hover:text-[#70b525]"
      : "text-gray-400 hover:text-gray-600";

  const downClasses =
    userVote === "down"
      ? "text-red-500 hover:text-red-600"
      : "text-gray-400 hover:text-gray-600";

  return (
    <div className="flex items-center gap-3 md:w-16 md:flex-col md:gap-2">
      <button
        type="button"
        onClick={() => {
          handleVote("up").catch(() => undefined);
        }}
        aria-pressed={userVote === "up"}
        aria-label="Positiv bewerten"
        className={`flex items-center gap-1 rounded-lg p-1 transition-colors ${upClasses}`}
      >
        <ThumbsUp
          className="h-4.5 w-4.5 md:h-5 md:w-5"
          fill={userVote === "up" ? "currentColor" : "none"}
        />
        <span
          className={`min-w-[2ch] text-center text-sm font-bold ${userVote === "up" ? "text-[#83CD2D]" : "text-gray-500"}`}
        >
          {upvotes}
        </span>
      </button>
      <button
        type="button"
        onClick={() => {
          handleVote("down").catch(() => undefined);
        }}
        aria-pressed={userVote === "down"}
        aria-label="Negativ bewerten"
        className={`flex items-center gap-1 rounded-lg p-1 transition-colors ${downClasses}`}
      >
        <ThumbsDown
          className="h-4.5 w-4.5 md:h-5 md:w-5"
          fill={userVote === "down" ? "currentColor" : "none"}
        />
        <span
          className={`min-w-[2ch] text-center text-sm font-bold ${userVote === "down" ? "text-red-500" : "text-gray-500"}`}
        >
          {downvotes}
        </span>
      </button>
    </div>
  );
}
