import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { GroupDefinition } from "~/components/database/grouped-list";
import type { Teacher } from "@/lib/teacher-api";
import { StaffMasterDetail } from "./staff-master-detail";

const meta: Meta<typeof StaffMasterDetail> = {
  title: "components/teachers/StaffMasterDetail",
  component: StaffMasterDetail,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof StaffMasterDetail>;

const teacherA: Teacher = {
  id: "1",
  name: "Anna Müller",
  first_name: "Anna",
  last_name: "Müller",
  email: "anna.mueller@schule-example.de",
  role: "Erzieherin",
  account_role: "teacher",
  qualifications: "Erzieherin, Zusatzqualifikation Sprachförderung",
  tag_id: "AB12CD34",
  staff_notes: "Vertretung montags.",
  created_at: "2026-01-10T08:00:00Z",
  updated_at: "2026-03-01T09:30:00Z",
};

const teacherB: Teacher = {
  id: "2",
  name: "Ben Schmidt",
  first_name: "Ben",
  last_name: "Schmidt",
  account_role: "teacher",
};

const groupDefinitions: GroupDefinition<Teacher>[] = [
  {
    id: "teachers",
    title: "Personal",
    items: [teacherA, teacherB],
  },
];

const emptyGroupDefinitions: GroupDefinition<Teacher>[] = [
  {
    id: "teachers",
    title: "Personal",
    items: [],
  },
];

export const Selected: Story = {
  args: {
    groupDefinitions,
    selectedId: teacherA.id,
    selectedTeacher: teacherA,
    onSelect: () => {},
    onEditClick: () => {},
    onDeleteClick: () => {},
    onUpdateNotes: async () => {},
    onManageCaregiver: () => {},
    onManageMFA: () => {},
  },
};

export const NoSelection: Story = {
  args: {
    groupDefinitions,
    selectedId: null,
    selectedTeacher: null,
    onSelect: () => {},
    onEditClick: () => {},
    onDeleteClick: () => {},
    onUpdateNotes: async () => {},
  },
};

export const EmptyList: Story = {
  args: {
    groupDefinitions: emptyGroupDefinitions,
    selectedId: null,
    selectedTeacher: null,
    onSelect: () => {},
    onEditClick: () => {},
    onDeleteClick: () => {},
    onUpdateNotes: async () => {},
  },
};

export const MinimalTeacher: Story = {
  args: {
    groupDefinitions,
    selectedId: teacherB.id,
    selectedTeacher: teacherB,
    onSelect: () => {},
    onEditClick: () => {},
    onDeleteClick: () => {},
    onUpdateNotes: async () => {},
  },
};
