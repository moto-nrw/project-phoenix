import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import CombinedGroupForm from "./combined-group-form";

const meta = {
  title: "components/groups/CombinedGroupForm",
  component: CombinedGroupForm,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof CombinedGroupForm>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Create: Story = {
  args: {
    formTitle: "Neue Gruppenkombination",
    submitLabel: "Erstellen",
    isLoading: false,
    onSubmitAction: async () => {},
    onCancelAction: () => {},
  },
};

export const Edit: Story = {
  args: {
    formTitle: "Gruppenkombination bearbeiten",
    submitLabel: "Speichern",
    isLoading: false,
    initialData: {
      name: "Kombinierte Gruppe A/B",
      is_active: true,
      access_policy: "specific",
      valid_until: "2026-12-31",
      specific_group_id: "42",
    },
    onSubmitAction: async () => {},
    onCancelAction: () => {},
  },
};

export const Loading: Story = {
  args: {
    formTitle: "Neue Gruppenkombination",
    submitLabel: "Erstellen",
    isLoading: true,
    onSubmitAction: async () => {},
    onCancelAction: () => {},
  },
};
