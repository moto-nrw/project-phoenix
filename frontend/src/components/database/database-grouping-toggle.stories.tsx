import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { DatabaseGroupingToggle } from "./database-grouping-toggle";

type GroupKey = "none" | "group" | "class";

const options: { value: GroupKey; label: string }[] = [
  { value: "none", label: "Keine Gruppierung" },
  { value: "group", label: "Nach Gruppe" },
  { value: "class", label: "Nach Klasse" },
];

const meta: Meta<typeof DatabaseGroupingToggle<GroupKey>> = {
  title: "components/database/DatabaseGroupingToggle",
  component: DatabaseGroupingToggle<GroupKey>,
};

export default meta;
type Story = StoryObj<typeof DatabaseGroupingToggle<GroupKey>>;

export const Default: Story = {
  render: () => {
    function Wrapper() {
      const [value, setValue] = useState<GroupKey>("none");
      return (
        <DatabaseGroupingToggle
          value={value}
          options={options}
          onChange={setValue}
        />
      );
    }
    return <Wrapper />;
  },
};

export const GroupSelected: Story = {
  render: () => {
    function Wrapper() {
      const [value, setValue] = useState<GroupKey>("group");
      return (
        <DatabaseGroupingToggle
          value={value}
          options={options}
          onChange={setValue}
        />
      );
    }
    return <Wrapper />;
  },
};
