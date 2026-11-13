import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Search } from "lucide-react";
import { type ComponentProps, useState } from "react";
import { expect, screen, userEvent, within } from "storybook/test";

import { SuggestInput } from "./suggest-input";

type StoryArgs = Omit<
  ComponentProps<typeof SuggestInput>,
  "value" | "onValueChange"
>;

function Controlled(args: StoryArgs) {
  const [value, setValue] = useState("");
  return (
    <div className="max-w-sm">
      <SuggestInput {...args} value={value} onValueChange={setValue} />
    </div>
  );
}

const meta = {
  title: "components/ui/SuggestInput",
  component: Controlled,
  args: {
    name: "position",
    label: "Position",
    listLabel: "Vorschläge für die Position",
    placeholder: "z. B. Pädagogische Fachkraft, OGS-Büro",
    suggestions: [
      "Pädagogische Fachkraft",
      "OGS-Büro",
      "Leitung",
      "Ergänzungskraft",
      "Hauswirtschaft",
    ],
  },
} satisfies Meta<typeof Controlled>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const CompactWithSearchIcon: Story = {
  args: {
    name: "activity",
    label: "Aktivität",
    listLabel: "Vorschläge für die Aktivität",
    placeholder: "Aktivität suchen oder neu eingeben",
    controlSize: "compact",
    leadingIcon: <Search />,
    highlightFirst: true,
    maxSuggestions: 8,
    suggestions: ["Basteln", "Fußball", "Hausaufgaben", "Lesen", "Malen"],
  },
};

export const WithHint: Story = {
  args: {
    hint: "Nur zur Anzeige in der Personalliste. Ändert keine Rechte.",
  },
};

export const WithError: Story = {
  args: {
    required: true,
    error: "Bitte geben Sie eine Position ein.",
  },
};

/** Offene Vorschlagsliste nach einer Eingabe (#6576, Baseline des Pixel-Vergleichs). */
export const Opened: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.type(
      within(canvasElement).getByRole("combobox", { name: "Position" }),
      "e",
    );
    await expect(
      await screen.findByRole("listbox", {
        name: "Vorschläge für die Position",
      }),
    ).toBeVisible();
  },
};
