import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Activity } from "@/lib/activity-helpers";
import { ActivitiesMasterDetail } from "./activities-master-detail";

const activities: Activity[] = [
  {
    id: "1",
    name: "Fußball AG",
    max_participant: 20,
    is_open_ags: true,
    supervisor_id: "s1",
    supervisor_name: "Anna Muster",
    ag_category_id: "c1",
    category_name: "Sport",
    created_at: new Date("2026-01-01"),
    updated_at: new Date("2026-01-01"),
  },
  {
    id: "2",
    name: "Schulhof Freispiel",
    max_participant: 0,
    is_open_ags: false,
    supervisor_id: "",
    ag_category_id: "",
    created_at: new Date("2026-01-01"),
    updated_at: new Date("2026-01-01"),
  },
];

const meta: Meta<typeof ActivitiesMasterDetail> = {
  title: "components/activities/ActivitiesMasterDetail",
  component: ActivitiesMasterDetail,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    activities,
    selectedId: null,
    selectedActivity: null,
    detailLoading: false,
    onSelect: () => {},
    onSaveActivity: async () => {},
    onResetForm: () => {},
    onDeleteClick: () => {},
    formResetKey: "reset-0",
  },
};

export default meta;
type Story = StoryObj<typeof ActivitiesMasterDetail>;

export const NoSelection: Story = {};

export const ActivitySelected: Story = {
  args: {
    selectedId: activities[0]?.id ?? null,
    selectedActivity: activities[0] ?? null,
  },
};

export const SystemActivitySelected: Story = {
  args: {
    selectedId: activities[1]?.id ?? null,
    selectedActivity: activities[1] ?? null,
  },
};

export const DetailLoading: Story = {
  args: {
    selectedId: activities[0]?.id ?? null,
    selectedActivity: activities[0] ?? null,
    detailLoading: true,
  },
};

export const Empty: Story = {
  args: {
    activities: [],
  },
};
