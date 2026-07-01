import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { AllowedDepartureModes } from "~/lib/student-helpers";

import { AllowedDepartureModesDisplay } from "./allowed-departure-modes-display";

const meta = {
  title: "students/AllowedDepartureModesDisplay",
  component: AllowedDepartureModesDisplay,
  parameters: {
    layout: "centered",
  },
} satisfies Meta<typeof AllowedDepartureModesDisplay>;

export default meta;

type Story = StoryObj<typeof meta>;

export const AlwaysAlone: Story = {
  args: {
    value: undefined,
  },
};

export const MixedArrangement: Story = {
  args: {
    value: {
      mon: ["bus"],
      tue: ["pickup"],
      wed: ["alone"],
      thu: ["bus", "pickup"],
      fri: ["accompanied"],
    } satisfies AllowedDepartureModes,
  },
};

export const SameEveryDay: Story = {
  args: {
    value: {
      mon: ["bus"],
      tue: ["bus"],
      wed: ["bus"],
      thu: ["bus"],
      fri: ["bus"],
    } satisfies AllowedDepartureModes,
  },
};
