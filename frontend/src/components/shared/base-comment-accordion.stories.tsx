import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import {
  BaseCommentAccordion,
  type BaseComment,
} from "./base-comment-accordion";

const sampleComments: BaseComment[] = [
  {
    id: "comment-1",
    content: "Vielen Dank für die schnelle Rückmeldung!",
    authorId: "author-1",
    authorName: "Anna Schmidt",
    authorType: "user",
    createdAt: "2026-06-20T09:15:00.000Z",
  },
  {
    id: "comment-2",
    content: "Wir prüfen das und melden uns bis morgen.",
    authorId: "author-2",
    authorName: "moto Support",
    authorType: "operator",
    createdAt: "2026-06-20T10:30:00.000Z",
  },
];

const meta: Meta<typeof BaseCommentAccordion> = {
  title: "shared/BaseCommentAccordion",
  component: BaseCommentAccordion,
  args: {
    postId: "post-1",
    loadComments: () => Promise.resolve(sampleComments),
    createComment: () => Promise.resolve(),
    deleteComment: () => Promise.resolve(),
  },
};

export default meta;
type Story = StoryObj<typeof BaseCommentAccordion>;

export const Default: Story = {};

export const WithCounts: Story = {
  args: {
    commentCount: 2,
    unreadCount: 1,
  },
};

export const Empty: Story = {
  args: {
    loadComments: () => Promise.resolve([]),
  },
};

export const RestrictedDelete: Story = {
  args: {
    canDeleteComment: (comment) => comment.authorType === "user",
  },
};
