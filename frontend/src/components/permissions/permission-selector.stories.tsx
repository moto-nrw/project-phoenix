import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { PermissionSelector } from "./permission-selector";

const meta = {
  title: "components/permissions/PermissionSelector",
  component: PermissionSelector,
  args: {
    value: undefined,
    onChange: fn(),
    required: false,
  },
} satisfies Meta<typeof PermissionSelector>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {};

export const Required: Story = {
  args: {
    required: true,
  },
};

export const ResourceSelected: Story = {
  args: {
    value: { resource: "users", action: "" },
  },
};

export const Complete: Story = {
  args: {
    value: { resource: "activities", action: "enroll" },
  },
};
