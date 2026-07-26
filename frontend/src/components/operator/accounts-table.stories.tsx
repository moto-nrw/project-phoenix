import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type {
  OrgAccount,
  School,
  SchoolAccount,
} from "~/lib/operator/provisioning-helpers";
import { AccountsTable } from "~/components/operator/accounts-table";

const schoolAccounts: SchoolAccount[] = [
  {
    accountId: "1",
    email: "anna.beispiel@schule.de",
    active: true,
    firstName: "Anna",
    lastName: "Beispiel",
    roleName: "Betreuung, Verwaltung",
    pedagogicRole: "Erzieherin",
    status: "active",
    hasAdminRole: true,
    hasUserRole: true,
    hasCaregiverProfile: true,
    isActiveCaregiver: true,
  },
  {
    accountId: "2",
    email: "max.muster@schule.de",
    active: true,
    firstName: "Max",
    lastName: "Muster",
    roleName: "Betreuung",
    pedagogicRole: "",
    status: "pending",
    hasAdminRole: false,
    hasUserRole: true,
    hasCaregiverProfile: false,
    isActiveCaregiver: false,
  },
  {
    accountId: "0",
    email: "eingeladen@schule.de",
    active: false,
    firstName: "",
    lastName: "",
    roleName: "",
    pedagogicRole: "",
    status: "invited",
    hasAdminRole: false,
    hasUserRole: false,
    hasCaregiverProfile: false,
    isActiveCaregiver: false,
  },
];

const orgAccounts: OrgAccount[] = schoolAccounts.map((account, index) => ({
  ...account,
  schoolId: `school-${index}`,
  schoolName: index === 0 ? "Grundschule Musterstadt" : "Schule am Berg",
}));

const selectedSchool: School = {
  id: "school-0",
  organizationId: "org-1",
  name: "Grundschule Musterstadt",
  slug: "musterstadt",
  subdomain: "musterstadt",
  address: "Musterstraße 1",
  city: "Musterstadt",
  zip: "12345",
  phone: "",
  email: "",
  active: true,
  hidden: false,
  deletedAt: null,
  createdAt: "2026-01-01T00:00:00.000Z",
  updatedAt: "2026-01-01T00:00:00.000Z",
};

const meta = {
  title: "components/operator/AccountsTable",
  component: AccountsTable,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof AccountsTable>;

export default meta;

type Story = StoryObj<typeof meta>;

export const SchoolScope: Story = {
  args: {
    accounts: schoolAccounts,
    showSchool: false,
    selectedSchool,
    onManageCaregiver: fn(),
    onManageMFA: fn(),
  },
};

export const OrganizationScope: Story = {
  args: {
    accounts: orgAccounts,
    showSchool: true,
    selectedSchool: null,
    onManageCaregiver: fn(),
    onManageMFA: fn(),
  },
};

export const Empty: Story = {
  args: {
    accounts: [],
    showSchool: false,
    selectedSchool: null,
  },
};
