import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { Teacher } from "~/lib/teacher-api";
import { StaffList } from "./staff-list";

const teachers: Teacher[] = [
  {
    id: "1",
    name: "Mila Muster",
    first_name: "Mila",
    last_name: "Muster",
    account_role: "admin",
    role: "Leitung",
  },
  {
    id: "2",
    name: "Jonas Weber",
    first_name: "Jonas",
    last_name: "Weber",
    account_role: "teacher",
    email: "jonas@example.test",
  },
];

const meta: Meta<typeof StaffList> = {
  title: "components/teachers/StaffList",
  component: StaffList,
  args: {
    groupDefinitions: [
      { id: "admin", title: "Admin", items: [teachers[0]!] },
      { id: "teacher", title: "Betreuung", items: [teachers[1]!] },
    ],
    objectHref: (teacher) => `/staff/${teacher.id}?from=%2Fdatabase%2Fpersonal`,
  },
};

export default meta;

type Story = StoryObj<typeof StaffList>;

export const GroupedByRole: Story = {};

export const Empty: Story = {
  args: { groupDefinitions: [] },
};
