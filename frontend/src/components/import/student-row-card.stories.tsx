import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StudentRowCard } from "./student-row-card";

const meta: Meta<typeof StudentRowCard> = {
  title: "components/import/StudentRowCard",
  component: StudentRowCard,
};

export default meta;
type Story = StoryObj<typeof StudentRowCard>;

export const New: Story = {
  args: {
    index: 0,
    student: {
      row: 1,
      status: "new",
      errors: [],
      first_name: "Anna",
      last_name: "Müller",
      meta: ["3a", "Sonnengruppe", "Frau Müller"],
    },
  },
};

export const Existing: Story = {
  args: {
    index: 1,
    student: {
      row: 2,
      status: "existing",
      errors: [],
      first_name: "Ben",
      last_name: "Schmidt",
      meta: ["4b", "Mondgruppe", "Herr Schmidt"],
    },
  },
};

export const Warning: Story = {
  args: {
    index: 2,
    student: {
      row: 3,
      status: "warning",
      errors: ["Klasse nicht gefunden"],
      first_name: "Clara",
      last_name: "Weber",
      meta: [],
    },
  },
};

export const ErrorState: Story = {
  args: {
    index: 3,
    student: {
      row: 4,
      status: "error",
      errors: ["Vorname fehlt", "Nachname fehlt"],
      first_name: "",
      last_name: "",
      meta: [],
    },
  },
};
