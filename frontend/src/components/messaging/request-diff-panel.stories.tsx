import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { RequestDiffPanel } from "./request-diff-panel";

const meta = {
  title: "components/messaging/RequestDiffPanel",
  component: RequestDiffPanel,
} satisfies Meta<typeof RequestDiffPanel>;

export default meta;

type Story = StoryObj<typeof meta>;

export const StaffReview: Story = {
  args: {
    heading: "Änderungen",
    diff: [
      { label: "Abholzeit", old: "16:00", new: "17:30" },
      { label: "Betreuungstag", old: "Montag", new: "Dienstag" },
    ],
  },
};

export const ParentRequest: Story = {
  args: {
    heading: "Ihre Änderungswünsche",
    diff: [{ label: "Notfallkontakt", old: "—", new: "Max Mustermann" }],
  },
};

export const Empty: Story = {
  args: {
    heading: "Änderungen",
    diff: [],
  },
};
