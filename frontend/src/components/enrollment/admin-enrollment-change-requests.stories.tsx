import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import {
  AdminEnrollmentChangeRequestDetail,
  AdminEnrollmentChangeRequestsList,
} from "~/components/enrollment/admin-enrollment-change-requests";

/**
 * Both components fetch their data client-side
 * (`listAdminEnrollmentChangeRequests` / `getAdminEnrollmentChangeRequest`).
 * There is no backend in Storybook, so the fetch fails and each component
 * renders its own graceful error state instead of crashing — these stories
 * document that fallback state.
 */
const listMeta: Meta<typeof AdminEnrollmentChangeRequestsList> = {
  title: "components/enrollment/AdminEnrollmentChangeRequestsList",
  component: AdminEnrollmentChangeRequestsList,
};

export default listMeta;
type ListStory = StoryObj<typeof AdminEnrollmentChangeRequestsList>;

export const Empty: ListStory = {};

export const DetailNotFound: StoryObj<
  typeof AdminEnrollmentChangeRequestDetail
> = {
  render: () => (
    <AdminEnrollmentChangeRequestDetail changeRequestId="storybook-change-request-id" />
  ),
};
