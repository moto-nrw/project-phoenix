import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { BooleanField } from "./boolean-field";

const meta = {
  title: "components/settings/fields/BooleanField",
  component: BooleanField,
} satisfies Meta<typeof BooleanField>;

export default meta;

type Story = StoryObj<typeof meta>;

export const On: Story = {
  args: {
    value: true,
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    ariaLabel: "Einstellung aktiviert",
  },
  render: function Render() {
    const [value, setValue] = useState(true);
    return (
      <BooleanField
        value={value}
        onChange={setValue}
        ariaLabel="Einstellung aktiviert"
      />
    );
  },
};

export const Off: Story = {
  args: {
    value: false,
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    ariaLabel: "Einstellung deaktiviert",
  },
  render: function Render() {
    const [value, setValue] = useState(false);
    return (
      <BooleanField
        value={value}
        onChange={setValue}
        ariaLabel="Einstellung deaktiviert"
      />
    );
  },
};

export const Disabled: Story = {
  args: {
    value: true,
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    disabled: true,
    ariaLabel: "Einstellung nicht bearbeitbar",
  },
  render: function Render() {
    const [value, setValue] = useState(true);
    return (
      <BooleanField
        value={value}
        onChange={setValue}
        disabled
        ariaLabel="Einstellung nicht bearbeitbar"
      />
    );
  },
};
