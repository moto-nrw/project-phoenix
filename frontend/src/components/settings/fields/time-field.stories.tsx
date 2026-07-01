import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { TimeField } from "./time-field";

function TimeFieldDemo({
  initialValue,
  emptyLabel,
  disabled,
}: Readonly<{
  initialValue: string;
  emptyLabel?: string;
  disabled?: boolean;
}>) {
  const [value, setValue] = useState(initialValue);
  return (
    <TimeField
      value={value}
      onChange={setValue}
      disabled={disabled}
      emptyLabel={emptyLabel}
    />
  );
}

const meta = {
  title: "components/settings/fields/TimeField",
  component: TimeField,
  args: {
    value: "08:00",
    onChange: () => {
      // no-op for story defaults
    },
  },
} satisfies Meta<typeof TimeField>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithValue: Story = {
  render: () => <TimeFieldDemo initialValue="08:00" />,
};

export const Empty: Story = {
  render: () => <TimeFieldDemo initialValue="" />,
};

export const EmptyWithLabel: Story = {
  render: () => <TimeFieldDemo initialValue="" emptyLabel="Jederzeit" />,
};

export const Disabled: Story = {
  render: () => <TimeFieldDemo initialValue="17:30" disabled />,
};

export const DisabledEmptyWithLabel: Story = {
  render: () => (
    <TimeFieldDemo initialValue="" emptyLabel="Jederzeit" disabled />
  ),
};
