import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StudentCommonFormSections } from "./student-common-form-sections";

const noopFieldChange = () => {
  // no-op for story
};

const meta = {
  title: "students/StudentCommonFormSections",
  component: StudentCommonFormSections,
} satisfies Meta<typeof StudentCommonFormSections>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    formData: {},
    errors: {},
    onChange: noopFieldChange,
  },
};

export const WithErrors: Story = {
  args: {
    formData: {
      health_info: "Erdnussallergie",
      supervisor_notes: "Braucht Erinnerung an Medikamente",
      extra_info: "Holt sich gerne Hilfe beim Anziehen",
    },
    errors: {
      data_retention_days: "Bitte gültige Anzahl an Tagen angeben",
    },
    onChange: noopFieldChange,
  },
};
