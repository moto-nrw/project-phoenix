import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Plus } from "lucide-react";

import { Button } from "./button";
import { Input } from "./input";

const meta = {
  title: "components/ui/Button",
  component: Button,
  args: {
    children: "Button",
  },
} satisfies Meta<typeof Button>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Primary: Story = {
  args: {
    variant: "primary",
  },
};

export const Secondary: Story = {
  args: {
    variant: "secondary",
  },
};

export const Outline: Story = {
  args: {
    variant: "outline",
  },
};

export const OutlineDanger: Story = {
  args: {
    variant: "outline_danger",
  },
};

export const Danger: Story = {
  args: {
    variant: "danger",
  },
};

export const Warning: Story = {
  args: {
    variant: "warning",
  },
};

export const Success: Story = {
  args: {
    variant: "success",
  },
};

export const Ghost: Story = {
  args: {
    variant: "ghost",
  },
};

export const GhostDanger: Story = {
  args: {
    variant: "ghost_danger",
    size: "md",
    children: "Entfernen",
  },
};

/** Leiser Knopf auf einer farbigen Meldung: nimmt deren Textfarbe an. */
export const GhostTone: Story = {
  args: {
    variant: "ghost_tone",
    size: "compact",
    type: "button",
    children: "Wiederholen",
  },
  decorators: [
    (Story) => (
      <div className="bg-moto-red-soft text-moto-red-strong rounded-xl p-4">
        <Story />
      </div>
    ),
  ],
};

/** Textknopf: kein Polster, sichtbarer Fokusring (Tab drücken). */
export const Link: Story = {
  args: {
    variant: "link",
    size: "md",
    type: "button",
    children: "Passwort vergessen?",
  },
};

/** Schwebender Anlegen-Knopf: rund, 56 px, grün. */
export const Fab: Story = {
  args: {
    variant: "fab",
    size: "icon",
    type: "button",
    "aria-label": "Aktivität erstellen",
    children: <Plus className="h-6 w-6" strokeWidth={2.5} aria-hidden />,
  },
};

/** Gestrichelte Hinzufügen-Fläche. */
export const Dashed: Story = {
  args: {
    variant: "dashed",
    size: "md",
    type: "button",
    className: "w-full gap-1.5",
    children: (
      <>
        <Plus className="h-4 w-4" aria-hidden />
        Weitere Nummer hinzufügen
      </>
    ),
  },
};

/** 40 px: bündig mit `Input controlSize="compact"` in derselben Zeile. */
export const FieldNextToInput: Story = {
  render: () => (
    <div className="flex max-w-md items-end gap-2">
      <div className="flex-1">
        <Input
          name="email"
          label="E-Mail"
          controlSize="compact"
          placeholder="name@schule.de"
        />
      </div>
      <Button type="button" size="field">
        Code senden
      </Button>
    </div>
  ),
};

export const Loading: Story = {
  args: {
    isLoading: true,
    loadingText: "Lädt...",
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};

export const Sizes: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button size="sm">sm</Button>
      <Button size="md">md</Button>
      <Button size="field">field</Button>
      <Button size="base">base</Button>
      <Button size="lg">lg</Button>
      <Button size="xl">xl</Button>
      <Button size="compact">compact</Button>
      <Button size="icon">i</Button>
    </div>
  ),
};
