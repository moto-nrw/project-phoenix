import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StatusBadge } from "./status-badge";
import { StatusColorBadge } from "./status-color-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";

const meta = {
  title: "components/ui/StatusBadge",
  component: StatusBadge,
} satisfies Meta<typeof StatusBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Green: Story = {
  args: { label: "Bestätigt", tone: "green" },
};

export const Blue: Story = {
  args: { label: "In Prüfung", tone: "blue" },
};

// "Needs attention": open, missing, pending. Yellow with a warning icon.
export const Warning: Story = {
  args: { label: "Antwort offen", tone: "warning" },
};

// The Schulhof hue; only for labels without a warning meaning.
export const Orange: Story = {
  args: { label: "Feiertag", tone: "orange" },
};

export const Red: Story = {
  args: { label: "Abgelehnt", tone: "red" },
};

export const Gray: Story = {
  args: { label: "Zurückgezogen", tone: "gray" },
};

export const Purple: Story = {
  args: { label: "Entschuldigt", tone: "purple" },
};

export const Cyan: Story = {
  args: { label: "Klassenfahrt", tone: "cyan" },
};

export const Outline: Story = {
  args: { label: "Entwurf", tone: "outline" },
};

export const SizeXs: Story = {
  args: { label: "Ausflug", tone: "cyan", size: "xs" },
};

export const AllTones: Story = {
  args: { label: "Bestätigt", tone: "green" },
  render: () => (
    <div className="flex flex-wrap gap-2">
      <StatusBadge label="In Prüfung" tone="blue" />
      <StatusBadge label="Bestätigt" tone="green" />
      <StatusBadge label="Antwort offen" tone="warning" />
      <StatusBadge label="Feiertag" tone="orange" />
      <StatusBadge label="Abgelehnt" tone="red" />
      <StatusBadge label="Zurückgezogen" tone="gray" />
      <StatusBadge label="Entschuldigt" tone="purple" />
      <StatusBadge label="Klassenfahrt" tone="cyan" />
      <StatusBadge label="Entwurf" tone="outline" />
    </div>
  ),
};

// #5617: an attention pill in a list next to the Schulhof location pill must
// not read as the same thing.
export const WarningNextToSchoolyard: Story = {
  args: { label: "Anfrage offen", tone: "warning" },
  render: () => (
    <ul className="w-72 divide-y divide-gray-100 text-sm">
      <li className="flex items-center justify-between py-2">
        <span>Mia</span>
        <StatusColorBadge label="Schulhof" color={LOCATION_COLORS.SCHOOLYARD} />
      </li>
      <li className="flex items-center justify-between py-2">
        <span>Ben</span>
        <StatusBadge label="Anfrage offen" tone="warning" />
      </li>
      <li className="flex items-center justify-between py-2">
        <span>Lea</span>
        <StatusColorBadge label="Fehlt" color={LOCATION_COLORS.WARNING} />
      </li>
    </ul>
  ),
};
