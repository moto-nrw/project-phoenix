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
      school_class: "3a",
      group_name: "Sonnengruppe",
      guardian_info: "Frau Müller",
      health_info: "",
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
      school_class: "4b",
      group_name: "Mondgruppe",
      guardian_info: "Herr Schmidt",
      health_info: "",
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
      school_class: "",
      group_name: "",
      guardian_info: "",
      health_info: "",
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
      school_class: "",
      group_name: "",
      guardian_info: "",
      health_info: "",
    },
  },
};
