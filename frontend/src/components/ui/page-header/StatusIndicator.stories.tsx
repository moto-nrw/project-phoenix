import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StatusIndicator } from "./StatusIndicator";

const meta = {
  title: "components/ui/page-header/StatusIndicator",
  component: StatusIndicator,
  args: {
    color: "green",
  },
} satisfies Meta<typeof StatusIndicator>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Green: Story = {
  args: {
    color: "green",
    tooltip: "Online",
  },
};

export const Yellow: Story = {
  args: {
    color: "yellow",
    tooltip: "Verbindung wird geprüft",
  },
};

export const Red: Story = {
  args: {
    color: "red",
    tooltip: "Offline",
  },
};

export const Gray: Story = {
  args: {
    color: "gray",
    tooltip: "Unbekannt",
  },
};

export const Large: Story = {
  args: {
    color: "green",
    size: "md",
    tooltip: "Online",
  },
};

export const Gallery: Story = {
  render: () => (
    <div className="flex items-center gap-4">
      <StatusIndicator color="green" tooltip="Online" />
      <StatusIndicator color="yellow" tooltip="Prüft" />
      <StatusIndicator color="red" tooltip="Offline" />
      <StatusIndicator color="gray" tooltip="Unbekannt" />
    </div>
  ),
};
