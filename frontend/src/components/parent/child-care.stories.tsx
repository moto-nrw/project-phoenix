import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { CareException, ChildFeatures, StatusDay } from "~/lib/parent-api";
import {
  CareScheduleRequestModal,
  PickupTimeModal,
  RequestChooserModal,
  SickNoteModal,
  SickStatusSummary,
} from "./child-care";

const ALL_FEATURES: ChildFeatures = {
  sick_note_enabled: true,
  notes_enabled: true,
  request_submit_enabled: true,
  pickup_change_enabled: true,
  related_accounts_invite_enabled: true,
  related_accounts_remove_enabled: true,
  master_data_edit_enabled: true,
  master_data_contact_edit_enabled: true,
  master_data_request_enabled: true,
  meal_plan_enabled: true,
  parent_news_enabled: true,
};

const CARE_EXCEPTIONS: CareException[] = [
  {
    date: "2026-07-10",
    pickup_time: "15:30",
    source: "guardian",
    updated_at: "2026-07-01T10:00:00Z",
  },
];

const SICK_DAYS: StatusDay[] = [
  {
    id: "1",
    student_id: "1",
    date: "2026-07-01",
    status: "sick",
    reported_at: "2026-07-01T08:00:00Z",
    source: "guardian",
  },
  {
    id: "2",
    student_id: "1",
    date: "2026-07-02",
    status: "sick",
    reported_at: "2026-07-01T08:00:00Z",
    source: "guardian",
  },
];

const noopSubmit = async () => undefined;
const noopRemove = async () => undefined;

const meta = {
  title: "components/parent/ChildCare",
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const SickNote: Story = {
  render: () => (
    <SickNoteModal onClose={() => undefined} onSubmit={noopSubmit} />
  ),
};

export const PickupTimeEmpty: Story = {
  render: () => (
    <PickupTimeModal
      careExceptions={[]}
      careExceptionsLoaded={true}
      pickupChangeEnabled={true}
      onClose={() => undefined}
      onSubmit={noopSubmit}
      onRemove={noopRemove}
    />
  ),
};

export const PickupTimeWithExistingOverride: Story = {
  render: () => (
    <PickupTimeModal
      careExceptions={CARE_EXCEPTIONS}
      careExceptionsLoaded={true}
      pickupChangeEnabled={true}
      onClose={() => undefined}
      onSubmit={noopSubmit}
      onRemove={noopRemove}
    />
  ),
};

export const PickupTimeNotLoaded: Story = {
  render: () => (
    <PickupTimeModal
      careExceptions={[]}
      careExceptionsLoaded={false}
      pickupChangeEnabled={true}
      onClose={() => undefined}
      onSubmit={noopSubmit}
      onRemove={noopRemove}
    />
  ),
};

export const StatusSummaryEmpty: Story = {
  render: () => <SickStatusSummary sickDays={[]} />,
};

export const StatusSummaryRange: Story = {
  render: () => <SickStatusSummary sickDays={SICK_DAYS} />,
};

export const CareScheduleRequest: Story = {
  render: () => (
    <CareScheduleRequestModal onClose={() => undefined} onSubmit={noopSubmit} />
  ),
};

export const RequestChooser: Story = {
  render: () => (
    <RequestChooserModal
      features={ALL_FEATURES}
      onPick={() => undefined}
      onClose={() => undefined}
    />
  ),
};
