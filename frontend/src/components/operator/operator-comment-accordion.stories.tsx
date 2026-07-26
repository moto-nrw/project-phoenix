import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { OperatorCommentAccordion } from "./operator-comment-accordion";

const meta: Meta<typeof OperatorCommentAccordion> = {
  title: "operator/OperatorCommentAccordion",
  component: OperatorCommentAccordion,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof OperatorCommentAccordion>;

export const Default: Story = {
  args: {
    postId: "post-1",
  },
};

export const WithCommentCount: Story = {
  args: {
    postId: "post-2",
    commentCount: 3,
  },
};

export const WithUnreadComments: Story = {
  args: {
    postId: "post-3",
    commentCount: 5,
    unreadCount: 2,
  },
};

export const IsNew: Story = {
  args: {
    postId: "post-4",
    commentCount: 1,
    isNew: true,
  },
};
