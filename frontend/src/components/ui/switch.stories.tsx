import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { Switch, SwitchRow } from "./switch";

const meta = {
  title: "components/ui/Switch",
  component: Switch,
} satisfies Meta<typeof Switch>;

export default meta;

type Story = StoryObj<typeof meta>;

export const On: Story = {
  args: {
    checked: true,
    onCheckedChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    ariaLabel: "Einstellung aktiviert",
  },
  render: function Render() {
    const [value, setValue] = useState(true);
    return (
      <Switch
        checked={value}
        onCheckedChange={setValue}
        ariaLabel="Einstellung aktiviert"
      />
    );
  },
};

export const Off: Story = {
  args: {
    checked: false,
    onCheckedChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    ariaLabel: "Einstellung deaktiviert",
  },
  render: function Render() {
    const [value, setValue] = useState(false);
    return (
      <Switch
        checked={value}
        onCheckedChange={setValue}
        ariaLabel="Einstellung deaktiviert"
      />
    );
  },
};

export const Disabled: Story = {
  args: {
    checked: true,
    onCheckedChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    disabled: true,
    ariaLabel: "Einstellung nicht bearbeitbar",
  },
  render: function Render() {
    const [value, setValue] = useState(true);
    return (
      <Switch
        checked={value}
        onCheckedChange={setValue}
        disabled
        ariaLabel="Einstellung nicht bearbeitbar"
      />
    );
  },
};

export const Row: Story = {
  args: {
    checked: true,
    onCheckedChange: () => {
      // no-op for story fixture; render() below manages its own state
    },
    ariaLabel: "Halber Tag",
  },
  render: function Render() {
    const [value, setValue] = useState(true);
    return (
      <SwitchRow
        label="Halber Tag"
        description="Aktivieren, wenn nur ein halber Arbeitstag beansprucht wird."
        checked={value}
        onCheckedChange={setValue}
      />
    );
  },
};
