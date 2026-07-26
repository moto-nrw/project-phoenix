import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { User } from "lucide-react";

import { InfoCard, InfoItem } from "./info-card";

const meta: Meta<typeof InfoCard> = {
  title: "components/ui/InfoCard",
  component: InfoCard,
};

export default meta;

type Story = StoryObj<typeof InfoCard>;

export const Default: Story = {
  args: {
    title: "Stammdaten",
    icon: <User className="h-5 w-5" />,
    children: <p className="text-sm text-gray-600">Beispielinhalt</p>,
  },
};

export const WithInfoItems: Story = {
  args: {
    title: "Kontaktdaten",
    icon: <User className="h-5 w-5" />,
    children: (
      <>
        <InfoItem label="Name" value="Max Mustermann" />
        <InfoItem
          label="Telefon"
          value="0123 456789"
          icon={<User className="h-4 w-4" />}
        />
      </>
    ),
  },
};

export const InfoItemOnly: StoryObj<typeof InfoItem> = {
  render: () => <InfoItem label="Label" value="Wert" />,
};
