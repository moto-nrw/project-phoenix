import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AuthShell, OperatorBrand } from "./auth-shell";

const meta: Meta<typeof AuthShell> = {
  title: "components/auth/AuthShell",
  component: AuthShell,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof AuthShell>;

export const Tenant: Story = {
  args: {
    eyebrow: "Anmeldung",
    title: "Willkommen zurück",
    subtitle: "Melde dich mit deinen Zugangsdaten an.",
    variant: "tenant",
    children: <div className="text-sm text-gray-500">Formularinhalt hier</div>,
  },
};

export const Parents: Story = {
  args: {
    eyebrow: "Elternportal",
    title: "Anmelden",
    subtitle: "Zugriff auf alle Informationen rund um dein Kind.",
    variant: "parents",
    children: <div className="text-sm text-gray-500">Formularinhalt hier</div>,
  },
};

export const WithBrandAndFooter: Story = {
  args: {
    eyebrow: "Operator",
    title: "Zugang zur Plattform",
    subtitle: "Nur für autorisierte Mitarbeitende.",
    variant: "operator",
    brand: <OperatorBrand />,
    footer: (
      <p className="text-xs text-gray-500">
        Noch keinen Zugang? Kontakt aufnehmen.
      </p>
    ),
    children: <div className="text-sm text-gray-500">Formularinhalt hier</div>,
  },
};

export const OperatorBrandOnly: StoryObj<typeof OperatorBrand> = {
  render: () => <OperatorBrand />,
};
