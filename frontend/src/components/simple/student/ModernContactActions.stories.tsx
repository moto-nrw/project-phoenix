import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ModernContactActions } from "./ModernContactActions";

const meta = {
  title: "simple/student/ModernContactActions",
  component: ModernContactActions,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof ModernContactActions>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NoContact: Story = {
  args: {},
};

export const EmailOnly: Story = {
  args: {
    email: "eltern@example.com",
    studentName: "Max Mustermann",
  },
};

export const SinglePhone: Story = {
  args: {
    email: "eltern@example.com",
    phone: "0176 12345678",
    studentName: "Max Mustermann",
  },
};

export const MultiplePhones: Story = {
  args: {
    email: "eltern@example.com",
    studentName: "Max Mustermann",
    phoneNumbers: [
      { number: "0176 12345678", label: "Mutter", isPrimary: true },
      { number: "0170 98765432", label: "Vater" },
      { number: "030 1234567", label: "Notfall" },
    ],
  },
};
