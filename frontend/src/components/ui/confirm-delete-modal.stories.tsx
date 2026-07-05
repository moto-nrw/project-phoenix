import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { ConfirmDeleteModal } from "./confirm-delete-modal";

const meta = {
  title: "components/ui/ConfirmDeleteModal",
  component: ConfirmDeleteModal,
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof ConfirmDeleteModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const TwoStep: Story = {
  args: {
    isOpen: true,
    title: "Gerät löschen",
    description: "Möchtest du dieses Gerät wirklich unwiderruflich löschen?",
    gate: { mode: "twoStep" },
    onConfirm: () => {},
    onClose: () => {},
    loading: false,
    error: "",
  },
};

export const TextConfirm: Story = {
  args: {
    isOpen: true,
    title: "Person löschen",
    description:
      "Dieser Vorgang anonymisiert alle Daten der Person und kann nicht rückgängig gemacht werden.",
    gate: {
      mode: "textConfirm",
      expected: "LÖSCHEN",
      inputId: "confirm-delete-input",
      label: "Gib LÖSCHEN ein, um zu bestätigen",
      placeholder: "LÖSCHEN",
    },
    onConfirm: () => {},
    onClose: () => {},
    loading: false,
    error: "",
  },
};

export const WithError: Story = {
  args: {
    isOpen: true,
    title: "Gerät löschen",
    description: "Möchtest du dieses Gerät wirklich unwiderruflich löschen?",
    gate: { mode: "twoStep", firstStepLabel: "Weiter" },
    onConfirm: () => {},
    onClose: () => {},
    loading: false,
    error: "Das Gerät konnte nicht gelöscht werden.",
  },
};

export const Loading: Story = {
  args: {
    isOpen: true,
    title: "Gerät löschen",
    description: "Möchtest du dieses Gerät wirklich unwiderruflich löschen?",
    gate: { mode: "twoStep" },
    confirmDisabled: true,
    onConfirm: () => {},
    onClose: () => {},
    loading: true,
    error: "",
  },
};

export const Interactive: Story = {
  args: {
    isOpen: true,
    title: "Mitarbeiter löschen",
    description: "Diese Aktion kann nicht rückgängig gemacht werden.",
    gate: {
      mode: "textConfirm",
      expected: "LÖSCHEN",
      inputId: "confirm-delete-input-interactive",
      label: "Gib LÖSCHEN ein, um zu bestätigen",
    },
    onConfirm: () => {},
    onClose: () => {},
    loading: false,
    error: "",
  },
  render: () => {
    function InteractiveDemo() {
      const [isOpen, setIsOpen] = useState(true);
      const [loading, setLoading] = useState(false);

      const handleConfirm = async () => {
        setLoading(true);
        await new Promise((resolve) => setTimeout(resolve, 500));
        setLoading(false);
        setIsOpen(false);
      };

      return (
        <ConfirmDeleteModal
          isOpen={isOpen}
          title="Mitarbeiter löschen"
          description="Diese Aktion kann nicht rückgängig gemacht werden."
          gate={{
            mode: "textConfirm",
            expected: "LÖSCHEN",
            inputId: "confirm-delete-input-interactive",
            label: "Gib LÖSCHEN ein, um zu bestätigen",
          }}
          onConfirm={handleConfirm}
          onClose={() => setIsOpen(false)}
          loading={loading}
          error=""
        />
      );
    }

    return <InteractiveDemo />;
  },
};
