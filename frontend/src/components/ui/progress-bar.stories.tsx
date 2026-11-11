import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ProgressBar } from "./progress-bar";

const meta = {
  title: "components/ui/ProgressBar",
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const AllTones: Story = {
  render: () => (
    <div className="flex w-64 flex-col gap-3">
      <ProgressBar value={60} max={100} label="Grün" tone="green" />
      <ProgressBar value={60} max={100} label="Orange" tone="orange" />
      <ProgressBar value={60} max={100} label="Grau" tone="gray" />
      <ProgressBar value={60} max={100} label="Dunkel" tone="dark" />
      <ProgressBar value={60} max={100} label="Datenfarbe" color="#78716C" />
    </div>
  ),
};

export const Sizes: Story = {
  render: () => (
    <div className="flex w-64 flex-col gap-3">
      <ProgressBar value={40} max={100} label="xs" size="xs" />
      <ProgressBar value={40} max={100} label="sm" size="sm" />
      <ProgressBar value={40} max={100} label="md" size="md" />
    </div>
  ),
};
