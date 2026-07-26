import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Suggestion } from "~/lib/suggestions-helpers";
import { VoteButtons } from "./vote-buttons";

function makeSuggestion(overrides: Partial<Suggestion> = {}): Suggestion {
  return {
    id: "1",
    title: "Mehr Spielgeraete im Schulhof",
    description: "Es waeren mehr Spielgeraete im Schulhof wuenschenswert.",
    authorId: "1",
    authorName: "Max Mustermann",
    status: "open",
    score: 3,
    upvotes: 5,
    downvotes: 2,
    commentCount: 0,
    unreadCount: 0,
    userVote: null,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

const meta = {
  title: "suggestions/VoteButtons",
  component: VoteButtons,
  parameters: {
    layout: "centered",
  },
} satisfies Meta<typeof VoteButtons>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NoVote: Story = {
  args: {
    suggestion: makeSuggestion(),
    onVoteChange: () => undefined,
  },
};

export const Upvoted: Story = {
  args: {
    suggestion: makeSuggestion({ userVote: "up", upvotes: 6, downvotes: 2 }),
    onVoteChange: () => undefined,
  },
};

export const Downvoted: Story = {
  args: {
    suggestion: makeSuggestion({ userVote: "down", upvotes: 5, downvotes: 3 }),
    onVoteChange: () => undefined,
  },
};
