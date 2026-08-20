import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { MasterDataReviewItem } from "./master-data-review-item";
import type { StaffMasterDataChange } from "~/lib/master-data-review-api";

const row: StaffMasterDataChange = {
  id: "100",
  student_id: "42",
  first_name: "Lara",
  last_name: "Beispiel",
  target: "person",
  field_key: "first_name",
  old_value: "Lara",
  new_value: "Lea",
  status: "pending",
  created_at: "2026-06-24T12:00:00Z",
};

const meta = {
  title: "components/students/MasterDataReviewItem",
  component: MasterDataReviewItem,
  args: {
    row,
    onDecided: () => undefined,
  },
} satisfies Meta<typeof MasterDataReviewItem>;

export default meta;

type Story = StoryObj<typeof meta>;

// Deciding a card posts to "/api/students/master-data-change-requests/{id}".
// In Storybook there is no backend, so the request fails and the card
// gracefully falls into its error state (caught in a try/catch — no crash).
export const Default: Story = {};

export const DepartureModes: Story = {
  args: {
    row: {
      ...row,
      id: "101",
      field_key: "allowed_departure_modes",
      old_value: { mon: ["pickup"] },
      new_value: { mon: ["bus", "alone"] },
    },
  },
};
