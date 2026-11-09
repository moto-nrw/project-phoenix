import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { NumberStepper } from "./number-stepper";

const meta = {
  title: "UI/NumberStepper",
  component: NumberStepper,
} satisfies Meta<typeof NumberStepper>;

export default meta;

function Example() {
  const [value, setValue] = useState("15");
  const current = Number.parseInt(value, 10) || 0;
  return (
    <NumberStepper
      id="story-stepper"
      name="story-stepper"
      value={value}
      onChange={(event) => setValue(event.target.value)}
      onStep={(delta) => setValue(String(current + delta))}
      canDecrement={current > 1}
      decrementLabel="Einen weniger"
      incrementLabel="Einen mehr"
    />
  );
}

export const Default: StoryObj = { render: () => <Example /> };
