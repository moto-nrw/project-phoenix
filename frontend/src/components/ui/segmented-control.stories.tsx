import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import {
  SegmentedControl,
  type SegmentedControlItem,
} from "./segmented-control";

const meta = {
  title: "components/ui/SegmentedControl",
} satisfies Meta;

export default meta;
type Story = StoryObj;

type Mode = "onsite" | "home" | "absent" | "other";

const ITEMS: readonly SegmentedControlItem<Mode>[] = [
  { value: "onsite", label: "Vor Ort", tone: "green" },
  { value: "home", label: "Homeoffice", tone: "blue" },
  { value: "absent", label: "Abwesend", tone: "red" },
  { value: "other", label: "Sonstiges", tone: "neutral" },
];

const noop = () => undefined;

export const Joined: Story = {
  render: () => (
    <SegmentedControl
      items={ITEMS}
      value="onsite"
      onChange={noop}
      ariaLabel="Arbeitsort"
    />
  ),
};

// The active pill takes its item's tone; one control per tone.
export const PillTones: Story = {
  render: () => (
    <div className="flex flex-col gap-3">
      {ITEMS.map((item) => (
        <SegmentedControl
          key={item.value}
          variant="pills"
          items={ITEMS}
          value={item.value}
          onChange={noop}
          ariaLabel={`Arbeitsort ${item.label}`}
        />
      ))}
    </div>
  ),
};
