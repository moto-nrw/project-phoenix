import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { KontoTab } from "./konto-tab";

const meta: Meta<typeof KontoTab> = {
  title: "components/staff/KontoTab",
  component: KontoTab,
  args: {
    teacher: {
      id: "42",
      name: "Mila Muster",
      first_name: "Mila",
      last_name: "Muster",
      email: "mila@example.test",
      role: "Betreuung",
      account_role: "teacher",
      tag_id: "04A2B7C9",
      staff_notes: "Springt bei Vertretungen gerne ein.",
      qualifications: "Staatlich anerkannte Erzieherin",
      created_at: "2026-01-05T09:00:00Z",
      updated_at: "2026-09-01T14:30:00Z",
      account_id: 7,
    },
  },
};

export default meta;

type Story = StoryObj<typeof KontoTab>;

export const ReadOnly: Story = {};

export const WithManagement: Story = {
  args: {
    onUpdateNotes: async () => undefined,
    onManageRole: () => undefined,
    onManageMFA: () => undefined,
    onManageCaregiver: () => undefined,
  },
};
