import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { TimeField } from "./time-field";

function Demo({ initial }: { readonly initial: string }) {
  const [value, setValue] = useState(initial);
  return (
    <div className="max-w-xs p-6">
      <TimeField
        value={value}
        onChange={setValue}
        label="Beginn"
        hint="Uhrzeit im Format HH:MM"
        placeholder="08:00"
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
