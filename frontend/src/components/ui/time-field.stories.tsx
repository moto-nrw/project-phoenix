import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, screen, userEvent, within } from "storybook/test";

import { TimeField } from "./time-field";

function Demo({
  initial,
  required,
  invalid,
  inline,
}: {
  readonly initial: string;
  readonly required?: boolean;
  readonly invalid?: boolean;
  readonly inline?: boolean;
}) {
  const [value, setValue] = useState(initial);
  return (
    <div className="max-w-xs p-6">
      <TimeField
        value={value}
        onChange={setValue}
        label="Beginn"
        hint="Uhrzeit im Format HH:MM"
        placeholder="08:00"
        required={required}
        invalid={invalid}
        inline={inline}
      />
    </div>
  );
}

const meta = {
  title: "components/ui/TimeField",
  component: Demo,
} satisfies Meta<typeof Demo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = { args: { initial: "" } };

export const Filled: Story = { args: { initial: "13:30" } };

/** Pflichtfeld mit Fehlerrahmen. */
export const RequiredInvalid: Story = {
  args: { initial: "", required: true, invalid: true },
};

/** Neben einem anderen Feld: Beschriftung und Hinweis nur für Vorleser. */
export const Inline: Story = { args: { initial: "13:30", inline: true } };

/** Offene Uhrzeit-Auswahl (#6576, Baseline des Pixel-Vergleichs). */
export const Opened: Story = {
  args: { initial: "13:30" },
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement).getByRole("button", { name: "Uhrzeit auswählen" }),
    );
    await expect(
      await screen.findByRole("dialog", { name: "Uhrzeit auswählen" }),
    ).toBeVisible();
  },
};
