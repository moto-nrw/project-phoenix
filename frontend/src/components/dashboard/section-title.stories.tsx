import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { SectionTitle } from "./section-title";

const meta = {
  title: "components/dashboard/SectionTitle",
  component: SectionTitle,
} satisfies Meta<typeof SectionTitle>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    title: "Aktuelle Aktivitäten",
  },
};

export const LongTitle: Story = {
  args: {
    title: "Ein sehr langer Abschnittstitel für die Übersicht",
  },
};
