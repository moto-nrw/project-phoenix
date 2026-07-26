import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { Phase } from "~/lib/enrollment-phase-api";
import { RolloverForm } from "./rollover-form";

const sourcePhase: Phase = {
  id: "1",
  name: "Schuljahr 2025/26",
  kind: "school_year",
  service_start_date: "2025-08-01",
  service_end_date: "2026-07-31",
  enrollment_open_at: null,
  enrollment_close_at: null,
  form_schema_id: null,
  show_status_reason_to_parent: true,
  care_overflow_mode: "waitlist",
  care_offering_selection_mode: "optional",
  is_active: true,
  rollover_source_phase_id: null,
  rollover_mode: null,
  rollover_auto_approve: false,
  rollover_deadline: null,
  rollover_bumps_grade: true,
  created_at: "2025-06-01T00:00:00Z",
  updated_at: "2025-06-01T00:00:00Z",
};

const meta = {
  title: "components/enrollment/RolloverForm",
  component: RolloverForm,
  parameters: {
    layout: "padded",
  },
  args: {
    source: sourcePhase,
    onCancel: fn(),
    onSuccess: fn(),
  },
} satisfies Meta<typeof RolloverForm>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const HolidayPhase: Story = {
  args: {
    source: {
      ...sourcePhase,
      id: "2",
      name: "Ferienbetreuung Sommer 2025",
      kind: "holiday",
      service_start_date: "2025-07-01",
      service_end_date: "2025-08-15",
    },
  },
};
