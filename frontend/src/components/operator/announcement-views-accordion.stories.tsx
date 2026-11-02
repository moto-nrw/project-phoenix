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
  },
};
