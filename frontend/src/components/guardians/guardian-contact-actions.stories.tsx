import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { GuardianContactActions } from "./guardian-contact-actions";

const meta = {
  title: "guardians/GuardianContactActions",
  component: GuardianContactActions,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof GuardianContactActions>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NoContact: Story = {
  args: {},
};

export const EmailOnly: Story = {
  args: {
    email: "eltern@example.com",
    contactName: "Max Mustermann",
  },
};

export const SinglePhone: Story = {
  args: {
    email: "eltern@example.com",
    phone: "0176 12345678",
    contactName: "Max Mustermann",
  },
};

export const MultiplePhones: Story = {
  args: {
    email: "eltern@example.com",
    contactName: "Max Mustermann",
    phoneNumbers: [
      { number: "0176 12345678", label: "Mutter", isPrimary: true },
      { number: "0170 98765432", label: "Vater" },
      { number: "030 1234567", label: "Notfall" },
    ],
  },
};
