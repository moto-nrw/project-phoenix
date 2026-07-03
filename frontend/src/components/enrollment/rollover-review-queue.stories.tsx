import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { RolloverReviewQueue } from "~/components/enrollment/rollover-review-queue";

const meta: Meta<typeof RolloverReviewQueue> = {
  title: "components/enrollment/RolloverReviewQueue",
  component: RolloverReviewQueue,
};

export default meta;
type Story = StoryObj<typeof RolloverReviewQueue>;

/**
 * The component fetches its review queue client-side via
 * `listRolloverReview(phaseID)`. There is no backend in Storybook, so the
 * fetch fails and the component renders its own graceful error state
 * instead of crashing — this story documents that fallback state.
 */
export const FetchFailed: Story = {
  args: {
    phaseID: "storybook-phase-id",
    phaseName: "Anmeldephase 2026/27",
  },
};
