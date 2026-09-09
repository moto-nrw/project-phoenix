import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import {
  GuardianDeleteModal,
  type GuardianDeleteScope,
} from "./guardian-delete-modal";

const meta = {
  title: "components/guardians/GuardianDeleteModal",
  component: GuardianDeleteModal,
  parameters: { layout: "fullscreen" },
  args: {
    isOpen: true,
    guardianName: "Maria Muster",
    onClose: () => {},
    onConfirmUnlink: () => {},
    onConfirmFullDelete: () => {},
  },
} satisfies Meta<typeof GuardianDeleteModal>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Ohne Admin-Recht: nur das zweistufige Entfernen von diesem Kind. */
export const UnlinkOnly: Story = {
  args: { canFullDelete: false },
};

/** Admin: Reichweite im Dialog wählen (#3110). */
export const AdminScopeChoice: Story = {
  args: { canFullDelete: true },
  render: (args) => {
    function Demo() {
      const [scope, setScope] = useState<GuardianDeleteScope | null>(null);
      return (
        <GuardianDeleteModal
          {...args}
          scope={scope}
          onScopeChange={setScope}
          fullDeleteWarning={
            scope === "full"
              ? "Die Person ist mit 2 Kindern verknüpft und wird bei allen entfernt: Anna Muster, Ben Muster."
              : null
          }
        />
      );
    }
    return <Demo />;
  },
};

/** Vollständig löschen, Warnung wird noch geladen. */
export const FullDeleteLoadingWarning: Story = {
  args: { canFullDelete: true, scope: "full", isWarningLoading: true },
};

/** Vollständig löschen mit Warnung und Namenseingabe. */
export const FullDeleteWithWarning: Story = {
  args: {
    canFullDelete: true,
    scope: "full",
    fullDeleteWarning:
      "Die Person ist mit 2 Kindern verknüpft und wird bei allen entfernt: Anna Muster, Ben Muster.",
  },
};

export const Loading: Story = {
  args: { canFullDelete: false, isLoading: true },
};
