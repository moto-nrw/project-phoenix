import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import type { Suggestion } from "~/lib/suggestions-helpers";
import { SuggestionForm } from "./suggestion-form";

const meta: Meta<typeof SuggestionForm> = {
  title: "components/suggestions/SuggestionForm",
  component: SuggestionForm,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof SuggestionForm>;

const mockSuggestion: Suggestion = {
  id: "1",
  title: "PDF-Export für Vertretungsplan",
  description:
    "Es wäre hilfreich, den Vertretungsplan als PDF exportieren zu können.",
  authorId: "1",
  authorName: "Max Mustermann",
  status: "open",
  score: 3,
  upvotes: 3,
  downvotes: 0,
  commentCount: 0,
  unreadCount: 0,
  userVote: null,
  createdAt: new Date("2026-01-01T10:00:00Z").toISOString(),
  updatedAt: new Date("2026-01-01T10:00:00Z").toISOString(),
};

export const CreateNew: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    onSuccess: () => {},
    editSuggestion: null,
  },
};

export const EditExisting: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    onSuccess: () => {},
    editSuggestion: mockSuggestion,
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    onClose: () => {},
    onSuccess: () => {},
    editSuggestion: null,
  },
};
