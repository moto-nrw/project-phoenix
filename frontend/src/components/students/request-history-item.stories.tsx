import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { RequestHistoryItem } from "./request-history-item";

const meta = {
  title: "components/students/RequestHistoryItem",
  component: RequestHistoryItem,
  args: {
    item: {
      request_type: "master_data",
      occurred_at: "2026-10-23T10:00:00Z",
      data: {
        id: "100",
        student_id: "42",
        first_name: "Testkind",
        last_name: "Beispiel",
        target: "student",
        field_key: "health_info",
        old_value: "Angabe vorhanden",
        new_value: "Angabe geändert",
        status: "auto_applied",
        created_at: "2026-10-23T10:00:00Z",
        decided_at: "2026-10-23T10:00:00Z",
      },
    },
  },
} satisfies Meta<typeof RequestHistoryItem>;

export default meta;
type Story = StoryObj<typeof meta>;

export const HealthChanged: Story = {};

export const HealthAdded: Story = {
  args: {
    item: {
      ...meta.args.item,
      data: {
        ...meta.args.item.data,
        old_value: "",
        new_value: "Angabe hinzugefügt",
      },
    },
  },
};

export const HealthRemoved: Story = {
  args: {
    item: {
      ...meta.args.item,
      data: { ...meta.args.item.data, new_value: "" },
    },
  },
};
