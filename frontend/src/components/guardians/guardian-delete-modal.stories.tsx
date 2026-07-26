import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { GuardianDeleteModal } from "./guardian-delete-modal";

const meta: Meta<typeof GuardianDeleteModal> = {
  title: "components/guardians/GuardianDeleteModal",
  component: GuardianDeleteModal,
  args: {
    isOpen: true,
    guardianName: "Maria Musterfrau",
    onClose: () => undefined,
    onSelectUnlink: () => undefined,
    onSelectFullDelete: () => undefined,
    onConfirmUnlink: () => undefined,
    onConfirmFullDelete: () => undefined,
    onBack: () => undefined,
  },
};

export default meta;

type Story = StoryObj<typeof GuardianDeleteModal>;

export const ChooseNonAdmin: Story = {
  args: {
    step: "choose",
    canFullDelete: false,
  },
};

export const ChooseAdmin: Story = {
  args: {
    step: "choose",
    canFullDelete: true,
  },
};

export const ConfirmUnlink: Story = {
  args: {
    step: "confirm-unlink",
    canFullDelete: true,
  },
};

export const ConfirmUnlinkLoading: Story = {
  args: {
    step: "confirm-unlink",
    canFullDelete: true,
    isLoading: true,
  },
};

export const ConfirmFullWarningLoading: Story = {
  args: {
    step: "confirm-full",
    canFullDelete: true,
    isWarningLoading: true,
  },
};

export const ConfirmFullWithWarning: Story = {
  args: {
    step: "confirm-full",
    canFullDelete: true,
    fullDeleteWarning:
      "Diese Person wird bei 2 Kindern (Anna Beispiel, Ben Beispiel) entfernt.",
  },
};

export const ConfirmFullNoWarning: Story = {
  args: {
    step: "confirm-full",
    canFullDelete: true,
  },
};
