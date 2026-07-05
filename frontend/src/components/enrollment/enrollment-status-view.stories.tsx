import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { EnrollmentStatusView } from "~/components/enrollment/enrollment-status-view";

const meta: Meta<typeof EnrollmentStatusView> = {
  title: "components/enrollment/EnrollmentStatusView",
  component: EnrollmentStatusView,
};

export default meta;

type Story = StoryObj<typeof EnrollmentStatusView>;

// EnrollmentStatusView fetches its data from the backend on mount via
// fetchStatus()/listEnrollmentChangeRequests(). There is no mock server in
// this Storybook setup, so these calls fail in the browser and the component
// falls into its own error UI (loading -> error state) — this still exercises
// the real component tree, hooks, and translated copy without fabricating a
// StatusResponse fixture.
export const Default: Story = {
  args: {
    token: "storybook-token",
  },
};

export const JustSubmitted: Story = {
  args: {
    token: "storybook-token",
    justSubmitted: true,
  },
};
