import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { WeekdayToggleGroup } from "./weekday-toggle-group";

const DAYS = [
  { value: 1, label: "Mo" },
  { value: 2, label: "Di" },
  { value: 3, label: "Mi" },
  { value: 4, label: "Do" },
  { value: 5, label: "Fr" },
  { value: 6, label: "Sa" },
  { value: 7, label: "So" },
];

const meta = {
  title: "UI/WeekdayToggleGroup",
  component: WeekdayToggleGroup,
} satisfies Meta<typeof WeekdayToggleGroup>;

export default meta;

function Example() {
  const [value, setValue] = useState<number[]>([1, 3]);
  return (
    <WeekdayToggleGroup
      ariaLabel="Wochentage"
      days={DAYS}
      value={value}
      onToggle={(day) =>
        setValue((current) =>
          current.includes(day)
            ? current.filter((entry) => entry !== day)
            : [...current, day],
        )
      }
    />
  );
}

export const Default: StoryObj = { render: () => <Example /> };

/** Hinweis und Fehler unter den Tagen; ein Feiertag ist gesperrt. */
export const WithHintAndError: StoryObj = {
  render: () => (
    <WeekdayToggleGroup
      ariaLabel="Wochentage"
      days={DAYS.map((day) =>
        day.value === 3 ? { ...day, disabled: true } : day,
      )}
      value={[1]}
      onToggle={() => undefined}
      hint="Mittwoch ist ein Feiertag."
      error="Bitte wählen Sie mindestens zwei Tage."
      name="weekdays"
    />
  ),
};
