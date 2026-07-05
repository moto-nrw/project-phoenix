import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PasswordField } from "./password-field";

const meta = {
  title: "components/settings/fields/PasswordField",
  component: PasswordField,
} satisfies Meta<typeof PasswordField>;

export default meta;

type Story = StoryObj<typeof meta>;

async function noopOnChange() {
  // no-op for story fixture
}

async function mockRevealFn(_key: string): Promise<string | null> {
  return "geheimerWert123";
}

export const NotSet: Story = {
  args: {
    hasValue: false,
    settingKey: "security.device_pin",
    onChange: noopOnChange,
  },
};

export const HasValue: Story = {
  args: {
    hasValue: true,
    settingKey: "security.device_pin",
    onChange: noopOnChange,
    revealFn: mockRevealFn,
  },
};

export const PinPattern: Story = {
  args: {
    hasValue: true,
    settingKey: "devices.ogs_device_pin",
    onChange: noopOnChange,
    pattern: String.raw`^\d{4}$`,
    revealFn: mockRevealFn,
  },
};

export const Disabled: Story = {
  args: {
    hasValue: true,
    settingKey: "security.device_pin",
    onChange: noopOnChange,
    disabled: true,
  },
};
