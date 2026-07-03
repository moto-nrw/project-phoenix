import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { DatabaseSelect, GroupSelect } from "./database-select";

const sampleOptions = [
  { value: "room-1", label: "Raum 1" },
  { value: "room-2", label: "Raum 2" },
  { value: "room-3", label: "Raum 3 (deaktiviert)", disabled: true },
];

function DatabaseSelectDemo(
  props: Omit<
    React.ComponentProps<typeof DatabaseSelect>,
    "value" | "onChange"
  >,
) {
  const [value, setValue] = useState("");
  return <DatabaseSelect {...props} value={value} onChange={setValue} />;
}

const meta: Meta<typeof DatabaseSelect> = {
  title: "components/ui/database/DatabaseSelect",
  component: DatabaseSelect,
  parameters: {
    layout: "padded",
  },
};

export default meta;

type Story = StoryObj<typeof DatabaseSelect>;

export const Default: Story = {
  render: () => (
    <DatabaseSelectDemo name="room" label="Raum" options={sampleOptions} />
  ),
};

export const WithHelperText: Story = {
  render: () => (
    <DatabaseSelectDemo
      name="room"
      label="Raum"
      options={sampleOptions}
      helperText="Wähle den Raum für diese Gruppe aus."
    />
  ),
};

export const WithError: Story = {
  render: () => (
    <DatabaseSelectDemo
      name="room"
      label="Raum"
      options={sampleOptions}
      error="Dieses Feld ist erforderlich."
    />
  ),
};

export const Loading: Story = {
  render: () => (
    <DatabaseSelectDemo
      name="room"
      label="Raum"
      options={sampleOptions}
      loading
    />
  ),
};

export const Disabled: Story = {
  render: () => (
    <DatabaseSelectDemo
      name="room"
      label="Raum"
      options={sampleOptions}
      disabled
    />
  ),
};

export const NoOptions: Story = {
  render: () => (
    <DatabaseSelectDemo
      name="room"
      label="Raum"
      options={[]}
      includeEmpty={false}
    />
  ),
};

function GroupSelectDemo() {
  const [value, setValue] = useState("");
  return <GroupSelect name="group_id" value={value} onChange={setValue} />;
}

export const GroupSelectStory: StoryObj = {
  name: "GroupSelect",
  render: () => <GroupSelectDemo />,
};
