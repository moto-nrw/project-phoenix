import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { MasterDataReviewList } from "./master-data-review-list";

const meta = {
  title: "components/students/MasterDataReviewList",
  component: MasterDataReviewList,
} satisfies Meta<typeof MasterDataReviewList>;

export default meta;

type Story = StoryObj<typeof meta>;

// The component fetches from "/api/students/master-data-change-requests" on
// mount via a plain `fetch` call. In Storybook there is no backend, so the
// request fails and the component gracefully falls into its error state
// (caught in a try/catch — no crash).
export const Default: Story = {};
