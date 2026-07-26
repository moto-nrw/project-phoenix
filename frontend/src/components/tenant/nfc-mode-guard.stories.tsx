import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { NfcModeGuard } from "~/components/tenant/nfc-mode-guard";

const meta: Meta<typeof NfcModeGuard> = {
  title: "components/tenant/NfcModeGuard",
  component: NfcModeGuard,
};

export default meta;
type Story = StoryObj<typeof NfcModeGuard>;

export const NfcEnabled: Story = {
  args: {
    children: (
      <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
        Geschützter NFC-Inhalt
      </div>
    ),
  },
};
