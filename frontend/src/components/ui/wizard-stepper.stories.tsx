import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { WizardStepper } from "~/components/ui/wizard-stepper";

const meta: Meta<typeof WizardStepper> = {
  title: "components/ui/WizardStepper",
  component: WizardStepper,
};

export default meta;
type Story = StoryObj<typeof WizardStepper>;

const steps = ["Konto", "Schule", "Bestätigung"];

export const FirstStep: Story = {
  args: {
    steps,
    current: 0,
  },
};

export const MiddleStep: Story = {
  args: {
    steps,
    current: 1,
  },
};

export const LastStep: Story = {
  args: {
    steps,
    current: steps.length - 1,
  },
};
