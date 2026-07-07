import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ParentMessagesCard } from "./parent-messages-card";

/**
 * The SWR fetch in this component talks to `/api/messages/...`, which has no
 * backend in Storybook. The request fails, `onError` logs it, and the card
 * renders its empty-threads state — that is the state these stories capture.
 */
const meta: Meta<typeof ParentMessagesCard> = {
  title: "students/ParentMessagesCard",
  component: ParentMessagesCard,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof ParentMessagesCard>;

export const Default: Story = {
  args: {
    studentId: "1",
    studentName: "Max Mustermann",
  },
};

export const WithoutStudentName: Story = {
  args: {
    studentId: "1",
  },
};
