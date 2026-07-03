import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AnnouncementViewsAccordion } from "./announcement-views-accordion";

const meta: Meta<typeof AnnouncementViewsAccordion> = {
  title: "operator/AnnouncementViewsAccordion",
  component: AnnouncementViewsAccordion,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof AnnouncementViewsAccordion>;

export const Default: Story = {
  args: {
    announcementId: "announcement-1",
    dismissedCount: 3,
  },
};

export const SingleConfirmation: Story = {
  args: {
    announcementId: "announcement-2",
    dismissedCount: 1,
  },
};

export const NoConfirmations: Story = {
  args: {
    announcementId: "announcement-3",
    dismissedCount: 0,
  },
};
