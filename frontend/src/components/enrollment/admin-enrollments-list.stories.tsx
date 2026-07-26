import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AdminEnrollmentsList } from "~/components/enrollment/admin-enrollments-list";

const meta: Meta<typeof AdminEnrollmentsList> = {
  title: "components/enrollment/AdminEnrollmentsList",
  component: AdminEnrollmentsList,
};

export default meta;
type Story = StoryObj<typeof AdminEnrollmentsList>;

/**
 * The component fetches phases, requests, change requests, form schemas and
 * settings client-side on mount via several `~/lib/*-api` calls. There is no
 * backend in Storybook, so the requests fail and the component renders its
 * own graceful fallback state (empty setup guide + "Noch keine
 * Anmeldephase" empty state) instead of crashing.
 */
export const Default: Story = {};
