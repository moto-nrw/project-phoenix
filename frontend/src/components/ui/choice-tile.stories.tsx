import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { FileText, Upload } from "lucide-react";
import { useState } from "react";

import { Checkbox } from "./checkbox";
import { ChoiceTile } from "./choice-tile";
import { Radio } from "./radio";

const meta = {
  title: "components/ui/ChoiceTile",
  component: ChoiceTile,
} satisfies Meta<typeof ChoiceTile>;

export default meta;
type Story = StoryObj<typeof meta>;

export const CheckboxRows: Story = {
  render: () => {
    const [days, setDays] = useState<string[]>(["Montag"]);
    const toggle = (day: string) =>
      setDays((current) =>
        current.includes(day)
          ? current.filter((entry) => entry !== day)
          : [...current, day],
      );
    return (
      <div className="w-72 space-y-2">
        {["Montag", "Dienstag", "Mittwoch"].map((day) => (
          <ChoiceTile key={day} selected={days.includes(day)}>
            <Checkbox
              checked={days.includes(day)}
              onChange={() => toggle(day)}
            />
            {day}
          </ChoiceTile>
        ))}
        <ChoiceTile disabled>
          <Checkbox disabled />
          Donnerstag (kein Betreuungstag)
        </ChoiceTile>
      </div>
    );
  },
};

export const RadioRows: Story = {
  render: () => {
    const [value, setValue] = useState("text");
    return (
      <div className="w-72 space-y-2">
        <ChoiceTile selected={value === "text"} tone="blue">
          <Radio
            name="mode"
            checked={value === "text"}
            onChange={() => setValue("text")}
          />
          Text eingeben
        </ChoiceTile>
        <ChoiceTile selected={value === "pdf"} tone="blue">
          <Radio
            name="mode"
            checked={value === "pdf"}
            onChange={() => setValue("pdf")}
          />
          PDF hochladen
        </ChoiceTile>
      </div>
    );
  },
};

export const ButtonTiles: Story = {
  render: () => {
    const [mode, setMode] = useState<"text" | "pdf">("text");
    return (
      <div className="grid w-[32rem] gap-3 sm:grid-cols-2">
        <ChoiceTile
          as="button"
          tone="blue"
          selected={mode === "text"}
          aria-pressed={mode === "text"}
          onClick={() => setMode("text")}
          className="flex-col items-start gap-1 p-3"
        >
          <span className="flex items-center gap-2 font-semibold">
            <FileText className="h-4 w-4" aria-hidden="true" />
            Text eingeben
          </span>
          <span className="text-xs font-normal text-gray-500">
            Eltern lesen den Text direkt im Formular.
          </span>
        </ChoiceTile>
        <ChoiceTile
          as="button"
          tone="blue"
          selected={mode === "pdf"}
          aria-pressed={mode === "pdf"}
          onClick={() => setMode("pdf")}
          className="flex-col items-start gap-1 p-3"
        >
          <span className="flex items-center gap-2 font-semibold">
            <Upload className="h-4 w-4" aria-hidden="true" />
            PDF hochladen
          </span>
          <span className="text-xs font-normal text-gray-500">
            Eltern öffnen die Datei aus dem Formular.
          </span>
        </ChoiceTile>
      </div>
    );
  },
};

export const GreenSelection: Story = {
  render: () => (
    <div className="grid w-[32rem] gap-3 sm:grid-cols-2">
      <ChoiceTile as="button" tone="green" selected aria-pressed>
        Anwesenheitsliste
      </ChoiceTile>
      <ChoiceTile as="button" tone="green" aria-pressed={false}>
        Abholliste
      </ChoiceTile>
    </div>
  ),
};
