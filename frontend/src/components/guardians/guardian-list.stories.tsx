import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { GuardianWithRelationship } from "@/lib/guardian-helpers";
import GuardianList from "./guardian-list";

const primaryGuardian: GuardianWithRelationship = {
  id: "1",
  firstName: "Maria",
  lastName: "Musterfrau",
  email: "maria.musterfrau@example.com",
  phoneNumbers: [
    {
      id: "p1",
      phoneNumber: "0151 12345678",
      phoneType: "mobile",
      isPrimary: true,
      priority: 1,
    },
    {
      id: "p2",
      phoneNumber: "0221 987654",
      phoneType: "work",
      label: "Büro",
      isPrimary: false,
      priority: 2,
    },
  ],
  addressStreet: "Musterstraße 12",
  addressCity: "Köln",
  addressPostalCode: "50667",
  preferredContactMethod: "phone",
  languagePreference: "de",
  notes: "Bevorzugt Kontakt per Telefon am Nachmittag.",
  hasAccount: true,
  accountId: "acc-1",
  relationshipId: "r1",
  relationshipType: "mother",
  isPrimary: true,
  isEmergencyContact: true,
  canPickup: true,
  emergencyPriority: 1,
  accountStatus: "active",
};

const pendingGuardian: GuardianWithRelationship = {
  id: "2",
  firstName: "Thomas",
  lastName: "Beispiel",
  email: "thomas.beispiel@example.com",
  phoneNumbers: [],
  preferredContactMethod: "email",
  languagePreference: "en",
  hasAccount: false,
  relationshipId: "r2",
  relationshipType: "father",
  isPrimary: false,
  isEmergencyContact: false,
  canPickup: false,
  emergencyPriority: 2,
  accountStatus: "pending",
};

const noAccountGuardian: GuardianWithRelationship = {
  id: "3",
  firstName: "Lena",
  lastName: "Nachbarin",
  phoneNumbers: [],
  preferredContactMethod: "phone",
  languagePreference: "de",
  hasAccount: false,
  relationshipId: "r3",
  relationshipType: "other",
  isPrimary: false,
  isEmergencyContact: false,
  canPickup: false,
  emergencyPriority: 3,
  accountStatus: "none",
};

const meta: Meta<typeof GuardianList> = {
  title: "components/guardians/GuardianList",
  component: GuardianList,
  args: {
    onEdit: () => undefined,
    onInvite: () => undefined,
    invitingGuardianId: null,
    readOnly: false,
    showRelationship: true,
  },
};

export default meta;

type Story = StoryObj<typeof GuardianList>;

export const Empty: Story = {
  args: {
    guardians: [],
  },
};

export const MultipleGuardians: Story = {
  args: {
    guardians: [primaryGuardian, pendingGuardian, noAccountGuardian],
  },
};

export const Inviting: Story = {
  args: {
    guardians: [pendingGuardian],
    invitingGuardianId: pendingGuardian.id,
  },
};

export const ReadOnly: Story = {
  args: {
    guardians: [primaryGuardian, pendingGuardian],
    readOnly: true,
  },
};
