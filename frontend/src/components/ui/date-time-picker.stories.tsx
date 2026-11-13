import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, screen, userEvent, within } from "storybook/test";

import { DateTimePicker } from "./date-time-picker";

function Demo() {
  const [value, setValue] = useState("2026-09-09T10:30");
  return (
    <div className="max-w-md p-6">
      <DateTimePicker
        value={value}
        onChange={setValue}
        min="2026-09-09T09:00"
      />
    </div>
  );
}

const meta = {
  title: "components/ui/DateTimePicker",
  component: Demo,
} satisfies Meta<typeof Demo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Uhrzeit-Auswahl am Tag der Untergrenze (#6576, Baseline des Pixel-Vergleichs). */
export const TimeOpenedOnMinimumDay: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement).getByRole("button", { name: "Uhrzeit auswählen" }),
    );
    await expect(
      await screen.findByRole("dialog", { name: "Uhrzeit auswählen" }),
    ).toBeVisible();
  },
};
