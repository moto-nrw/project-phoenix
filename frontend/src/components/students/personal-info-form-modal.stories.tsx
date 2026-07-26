import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PersonalInfoFormModal } from "./personal-info-form-modal";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";

const baseStudent: ExtendedStudent = {
  id: "1",
  name: "Anna Beispiel",
  first_name: "Anna",
  second_name: "Beispiel",
  school_class: "3a",
  current_location: "Gruppenraum",
  bus: false,
};

const meta = {
  title: "students/PersonalInfoFormModal",
  component: PersonalInfoFormModal,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    student: baseStudent,
    onClose: () => {
      // no-op for story
    },
    onSave: async () => {
      // no-op for story
    },
  },
} satisfies Meta<typeof PersonalInfoFormModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {};

export const WithNotes: Story = {
  args: {
    student: {
      ...baseStudent,
      health_info: "Allergie gegen Nüsse",
      supervisor_notes: "Braucht Unterstützung bei den Hausaufgaben",
      extra_info: "Abholung nur durch Erziehungsberechtigte",
    },
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
