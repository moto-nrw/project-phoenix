import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import GuardianFormModal from "./guardian-form-modal";
import type { GuardianWithRelationship } from "@/lib/guardian-helpers";

const meta: Meta<typeof GuardianFormModal> = {
  title: "components/guardians/GuardianFormModal",
  component: GuardianFormModal,
  args: {
    isOpen: true,
    onClose: () => undefined,
    onSubmit: async () => undefined,
  },
};

export default meta;

type Story = StoryObj<typeof GuardianFormModal>;

export const Create: Story = {
  args: {
    mode: "create",
  },
};

const editGuardian: GuardianWithRelationship = {
  id: "1",
  firstName: "Maria",
  lastName: "Musterfrau",
  email: "maria.musterfrau@example.com",
  phoneNumbers: [
    {
      id: "p1",
      phoneNumber: "0170 1234567",
      phoneType: "mobile",
      label: "",
      isPrimary: true,
      priority: 0,
    },
  ],
  preferredContactMethod: "email",
  languagePreference: "de",
  hasAccount: false,
  relationshipId: "r1",
  relationshipType: "mother",
  guardianRole: "legal_guardian",
  isEmergencyContact: true,
  isPrimary: true,
  canPickup: true,
  emergencyPriority: 1,
  addressStreet: "Musterstraße 1",
  addressCity: "Musterstadt",
  addressPostalCode: "12345",
  notes: "",
};

export const Edit: Story = {
  args: {
    mode: "edit",
    initialData: editGuardian,
    onDelete: () => undefined,
  },
};
