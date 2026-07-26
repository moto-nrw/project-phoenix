import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Suggestion } from "~/lib/suggestions-helpers";

import { SuggestionCard } from "./suggestion-card";

const baseSuggestion: Suggestion = {
  id: "1",
  title: "Mehr Spielgeräte für den Schulhof",
  description:
    "Es wäre schön, wenn wir zusätzliche Spielgeräte für den Schulhof bekommen könnten, damit sich die Kinder in den Pausen besser beschäftigen können.",
  authorId: "42",
  authorName: "Anna Schmidt",
  status: "open",
  score: 3,
  upvotes: 5,
  downvotes: 2,
  commentCount: 4,
  unreadCount: 0,
  userVote: null,
  createdAt: "2026-06-01T10:00:00Z",
  updatedAt: "2026-06-01T10:00:00Z",
};

const meta = {
  title: "components/suggestions/SuggestionCard",
  component: SuggestionCard,
  args: {
    currentAccountId: "99",
    onEdit: () => undefined,
    onDelete: () => undefined,
    onVoteChange: () => undefined,
  },
} satisfies Meta<typeof SuggestionCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    suggestion: baseSuggestion,
  },
};

export const OwnedByCurrentUser: Story = {
  args: {
    suggestion: { ...baseSuggestion, authorId: "99" },
    currentAccountId: "99",
  },
};

export const PlannedStatus: Story = {
  args: {
    suggestion: { ...baseSuggestion, status: "planned" },
  },
};

export const WithUnreadComments: Story = {
  args: {
    suggestion: { ...baseSuggestion, commentCount: 6, unreadCount: 2 },
  },
};
