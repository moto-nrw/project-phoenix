import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { FormModal } from "./form-modal";
import { Button } from "./button";

const meta = {
  title: "components/ui/FormModal",
  component: FormModal,
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for stories
    },
    title: "Formular",
    children: <p>Dies ist der Inhalt des Formulars innerhalb des Modals.</p>,
  },
} satisfies Meta<typeof FormModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithFooter: Story = {
  args: {
    footer: (
      <>
        <Button type="button" variant="outline" size="md">
          Abbrechen
        </Button>
        <Button type="button" variant="primary" size="md">
          Speichern
        </Button>
      </>
    ),
  },
};

export const Small: Story = {
  args: {
    size: "sm",
    title: "Kleines Formular",
  },
};

export const ExtraLarge: Story = {
  args: {
    size: "xl",
    title: "Großes Formular",
  },
};

export const CenteredOnMobile: Story = {
  args: {
    mobilePosition: "center",
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
