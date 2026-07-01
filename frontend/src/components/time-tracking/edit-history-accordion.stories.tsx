import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { WorkSessionEdit } from "~/lib/time-tracking-helpers";

import { EditHistoryAccordion } from "./edit-history-accordion";

const meta: Meta<typeof EditHistoryAccordion> = {
  title: "components/time-tracking/EditHistoryAccordion",
  component: EditHistoryAccordion,
};

export default meta;
type Story = StoryObj<typeof EditHistoryAccordion>;

const sampleEdits: WorkSessionEdit[] = [
  {
    id: "1",
    sessionId: "10",
    staffId: "20",
    editedBy: "20",
    fieldName: "check_in_time",
    oldValue: "2026-06-10T07:00:00Z",
    newValue: "2026-06-10T07:15:00Z",
    notes: "Verspätung korrigiert",
    createdAt: "2026-06-10T09:00:00Z",
    editorName: "Anna Schmidt",
    isSelfEdit: true,
  },
  {
    id: "2",
    sessionId: "10",
    staffId: "20",
    editedBy: "20",
    fieldName: "check_out_time",
    oldValue: "2026-06-10T15:00:00Z",
    newValue: "2026-06-10T15:30:00Z",
    notes: "Verspätung korrigiert",
    createdAt: "2026-06-10T09:00:00Z",
    editorName: "Anna Schmidt",
    isSelfEdit: true,
  },
  {
    id: "3",
    sessionId: "10",
    staffId: "20",
    editedBy: "30",
    fieldName: "break_minutes",
    oldValue: "30",
    newValue: "45",
    notes: null,
    createdAt: "2026-06-11T10:00:00Z",
    editorName: "Max Müller",
    isSelfEdit: false,
  },
];

export const Loading: Story = {
  args: {
    edits: [],
    isLoading: true,
  },
};

export const Empty: Story = {
  args: {
    edits: [],
    isLoading: false,
  },
};

export const WithHistory: Story = {
  args: {
    edits: sampleEdits,
    isLoading: false,
  },
};

export const WithEditButton: Story = {
  args: {
    edits: sampleEdits,
    isLoading: false,
    onEdit: () => {
      // no-op for story
    },
  },
};
