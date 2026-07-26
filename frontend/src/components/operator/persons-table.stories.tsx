import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { PersonsTable } from "./persons-table";

const meta: Meta<typeof PersonsTable> = {
  title: "operator/PersonsTable",
  component: PersonsTable,
};

export default meta;
type Story = StoryObj<typeof PersonsTable>;

const persons: OperatorPerson[] = [
  {
    id: "1",
    firstName: "Anna",
    lastName: "Müller",
    fullName: "Anna Müller",
    hasAccount: true,
    schoolId: "1",
    schoolName: "Schule am Berg",
    organizationId: "1",
    organizationName: "Träger Nord",
    accountEmail: "anna.mueller@example.com",
    isStaff: true,
    isStudent: false,
    hasRfidCard: true,
    createdAt: "2026-01-15T08:00:00Z",
  },
  {
    id: "2",
    firstName: "Ben",
    lastName: "Schmidt",
    fullName: "Ben Schmidt",
    hasAccount: false,
    schoolId: "2",
    schoolName: "Schule am See",
    organizationId: "1",
    organizationName: "Träger Nord",
    accountEmail: null,
    isStaff: false,
    isStudent: true,
    hasRfidCard: false,
    createdAt: "2026-02-01T08:00:00Z",
  },
];

export const Empty: Story = {
  args: {
    persons: [],
  },
};

export const Default: Story = {
  args: {
    persons,
  },
};

export const WithSchoolColumn: Story = {
  args: {
    persons,
    showSchool: true,
  },
};

export const WithDeleteAction: Story = {
  args: {
    persons,
    onDelete: () => undefined,
  },
};
