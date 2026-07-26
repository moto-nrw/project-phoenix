import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { CommentAccordion } from "./comment-accordion";

const meta: Meta<typeof CommentAccordion> = {
  title: "suggestions/CommentAccordion",
  component: CommentAccordion,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof CommentAccordion>;

export const Default: Story = {
  args: {
    postId: "post-1",
    currentAccountId: "account-1",
  },
};

export const WithCommentCount: Story = {
  args: {
    postId: "post-2",
    currentAccountId: "account-1",
    commentCount: 3,
  },
};

export const WithUnreadComments: Story = {
  args: {
    postId: "post-3",
    currentAccountId: "account-1",
    commentCount: 5,
    unreadCount: 2,
  },
};
