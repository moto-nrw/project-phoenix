import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { SelectField } from "./select-field";

const options = [
  { label: "Option A", value: "a" },
  { label: "Option B", value: "b" },
  { label: "Option C", value: "c" },
];

const meta = {
  title: "components/settings/fields/SelectField",
  component: SelectField,
} satisfies Meta<typeof SelectField>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    value: "a",
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    options,
  },
  render: function Render() {
    const [value, setValue] = useState<unknown>("a");
    return <SelectField value={value} onChange={setValue} options={options} />;
  },
};

export const Disabled: Story = {
  args: {
    value: "b",
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    options,
    disabled: true,
  },
  render: function Render() {
    const [value, setValue] = useState<unknown>("b");
    return (
      <SelectField
        value={value}
        onChange={setValue}
        options={options}
        disabled
      />
    );
  },
};
