import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { TextField } from "./text-field";

const meta = {
  title: "components/settings/fields/TextField",
  component: TextField,
} satisfies Meta<typeof TextField>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    value: "Beispieltext",
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
  },
  render: function Render() {
    const [value, setValue] = useState("Beispieltext");
    return <TextField value={value} onChange={setValue} />;
  },
};

export const Empty: Story = {
  args: {
    value: "",
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
  },
  render: function Render() {
    const [value, setValue] = useState("");
    return <TextField value={value} onChange={setValue} />;
  },
};

export const Disabled: Story = {
  args: {
    value: "Nicht bearbeitbar",
    onChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    disabled: true,
  },
  render: function Render() {
    const [value, setValue] = useState("Nicht bearbeitbar");
    return <TextField value={value} onChange={setValue} disabled />;
  },
};
