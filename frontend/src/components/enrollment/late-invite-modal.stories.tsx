import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Phase } from "~/lib/enrollment-phase-api";
import { LateInviteModal } from "./phase-enrollment-actions";

const phase: Phase = {
  id: "phase-1",
  name: "Anmeldephase 2026/2027",
  kind: "school_year",
  service_start_date: "2026-08-01",
  service_end_date: "2027-07-31",
  enrollment_open_at: "2026-05-01T00:00:00Z",
  enrollment_close_at: "2026-06-15T00:00:00Z",
  form_schema_id: "schema-1",
  show_status_reason_to_parent: true,
  care_overflow_mode: "waitlist",
  care_offering_selection_mode: "exactly_one",
  is_active: true,
  created_at: "2026-04-01T00:00:00Z",
  updated_at: "2026-04-01T00:00:00Z",
};

const meta = {
  title: "components/enrollment/LateInviteModal",
  component: LateInviteModal,
  args: {
    isOpen: true,
    onClose: () => undefined,
    phase,
    phaseUrl: "https://schule.moto-app.de/enroll/phase-1",
  },
} satisfies Meta<typeof LateInviteModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
