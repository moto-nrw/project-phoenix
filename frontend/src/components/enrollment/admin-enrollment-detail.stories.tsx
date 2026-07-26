import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AdminEnrollmentDetail } from "~/components/enrollment/admin-enrollment-detail";

const meta: Meta<typeof AdminEnrollmentDetail> = {
  title: "components/enrollment/AdminEnrollmentDetail",
  component: AdminEnrollmentDetail,
};

export default meta;
type Story = StoryObj<typeof AdminEnrollmentDetail>;

/**
 * The component fetches its data client-side via `getAdminRequest(requestId)`.
 * There is no backend in Storybook, so the fetch fails and the component
 * renders its own graceful error state ("Anmeldung nicht gefunden.") instead
 * of crashing — this story documents that fallback state.
 */
export const RequestNotFound: Story = {
  args: {
    requestId: "storybook-request-id",
  },
};
