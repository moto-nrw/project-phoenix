import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { DateField } from "./date-field";

const meta = {
  title: "components/settings/fields/DateField",
  component: DateField,
} satisfies Meta<typeof DateField>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    value: "2026-06-10",
    onChange: () => undefined,
  },
};

export const Empty: Story = {
  args: {
    value: "",
    onChange: () => undefined,
  },
};

export const EmptyWithLabel: Story = {
  args: {
    value: "",
    onChange: () => undefined,
    emptyLabel: "Kein Datum",
  },
};

export const Disabled: Story = {
  args: {
    value: "2026-06-10",
    onChange: () => undefined,
    disabled: true,
  },
};

export const Interactive: Story = {
  args: {
    value: "2026-06-10",
    onChange: () => undefined,
  },
  render: () => {
    function InteractiveDateField() {
      const [value, setValue] = useState("2026-06-10");
      return <DateField value={value} onChange={setValue} />;
    }
    return <InteractiveDateField />;
  },
};
