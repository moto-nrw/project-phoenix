import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AdminEnrollmentChangeRequestDetail } from "~/components/enrollment/admin-enrollment-change-requests";

/**
 * Die Detailansicht lädt ihre Daten clientseitig
 * (`getAdminEnrollmentChangeRequest`). In Storybook gibt es kein Backend, der
 * Abruf schlägt also fehl und die Komponente rendert ihren Fehlerzustand —
 * genau den zeigt diese Story.
 */
const meta: Meta<typeof AdminEnrollmentChangeRequestDetail> = {
  title: "components/enrollment/AdminEnrollmentChangeRequestDetail",
  component: AdminEnrollmentChangeRequestDetail,
};

export default meta;

export const DetailNotFound: StoryObj<
  typeof AdminEnrollmentChangeRequestDetail
> = {
  render: () => (
    <AdminEnrollmentChangeRequestDetail changeRequestId="storybook-change-request-id" />
  ),
};
