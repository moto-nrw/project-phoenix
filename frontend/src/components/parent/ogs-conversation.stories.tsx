import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { OgsConversation } from "./ogs-conversation";

const meta = {
  title: "components/parent/OgsConversation",
  component: OgsConversation,
  args: {
    studentId: "1",
  },
} satisfies Meta<typeof OgsConversation>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithBackAndChild: Story = {
  args: {
    showBack: true,
    showChild: true,
  },
};
