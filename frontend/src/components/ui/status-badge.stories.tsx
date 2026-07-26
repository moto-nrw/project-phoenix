import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StatusBadge } from "./status-badge";

const meta = {
  title: "components/ui/StatusBadge",
  component: StatusBadge,
} satisfies Meta<typeof StatusBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Green: Story = {
  args: { label: "Bestätigt", tone: "green" },
};

export const Blue: Story = {
  args: { label: "In Prüfung", tone: "blue" },
};

export const Orange: Story = {
  args: { label: "Warteliste", tone: "orange" },
};

export const Red: Story = {
  args: { label: "Abgelehnt", tone: "red" },
};

export const Gray: Story = {
  args: { label: "Zurückgezogen", tone: "gray" },
};

export const AllTones: Story = {
  args: { label: "Bestätigt", tone: "green" },
  render: () => (
    <div className="flex flex-wrap gap-2">
      <StatusBadge label="In Prüfung" tone="blue" />
      <StatusBadge label="Bestätigt" tone="green" />
      <StatusBadge label="Warteliste" tone="orange" />
      <StatusBadge label="Abgelehnt" tone="red" />
      <StatusBadge label="Zurückgezogen" tone="gray" />
    </div>
  ),
};
