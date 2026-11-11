import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Drawer as DrawerPrimitive } from "vaul";

import { Button } from "~/components/ui/button";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";

const meta: Meta<typeof SlideOver> = {
  title: "components/ui/SlideOver",
  component: SlideOver,
};

export default meta;
type Story = StoryObj<typeof SlideOver>;

export const Default: Story = {
  render: () => (
    <SlideOver>
      <DrawerPrimitive.Trigger asChild>
        <Button type="button">Panel öffnen</Button>
      </DrawerPrimitive.Trigger>
      <SlideOverContent>
        <SlideOverHeader>
          <SlideOverTitle>Mensa</SlideOverTitle>
          <SlideOverDescription>Mittwoch, 24.09.2026</SlideOverDescription>
        </SlideOverHeader>
        <div className="flex-1 overflow-y-auto px-5 py-4 text-sm text-slate-600">
          Inhalt des Panels.
        </div>
        <SlideOverFooter>
          <DrawerPrimitive.Close asChild>
            <Button type="button" variant="outline">
              Schließen
            </Button>
          </DrawerPrimitive.Close>
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  ),
};

export const CustomWidth: Story = {
  render: () => (
    <SlideOver>
      <DrawerPrimitive.Trigger asChild>
        <Button type="button">Breites Panel öffnen</Button>
      </DrawerPrimitive.Trigger>
      <SlideOverContent size="md">
        <SlideOverHeader>
          <SlideOverTitle>Details</SlideOverTitle>
          <SlideOverDescription>
            Breiteres Panel für mehr Inhalt.
          </SlideOverDescription>
        </SlideOverHeader>
        <div className="flex-1 overflow-y-auto px-5 py-4 text-sm text-slate-600">
          Inhalt des breiteren Panels.
        </div>
      </SlideOverContent>
    </SlideOver>
  ),
};

export const WithFormError: Story = {
  render: () => (
    <SlideOver defaultOpen>
      <DrawerPrimitive.Trigger asChild>
        <Button type="button">Formular öffnen</Button>
      </DrawerPrimitive.Trigger>
      <SlideOverContent>
        <SlideOverHeader>
          <SlideOverTitle>Termin bearbeiten</SlideOverTitle>
        </SlideOverHeader>
        <SlideOverBody
          error="Bitte einen Titel eintragen."
          className="space-y-4 text-sm text-slate-600"
        >
          <p>Der Fehler steht oben im Rumpf, nicht als Toast.</p>
        </SlideOverBody>
        <SlideOverFooter>
          <Button type="button">Speichern</Button>
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  ),
};

/** Footer with two actions: shows the spacing between them (#6567). */
export const WithFooterActions: Story = {
  render: () => (
    <SlideOver defaultOpen>
      <DrawerPrimitive.Trigger asChild>
        <Button type="button">Formular öffnen</Button>
      </DrawerPrimitive.Trigger>
      <SlideOverContent>
        <SlideOverHeader>
          <SlideOverTitle>Termin bearbeiten</SlideOverTitle>
        </SlideOverHeader>
        <SlideOverBody className="text-sm text-slate-600">
          Inhalt des Panels.
        </SlideOverBody>
        <SlideOverFooter>
          <DrawerPrimitive.Close asChild>
            <Button type="button" variant="outline">
              Abbrechen
            </Button>
          </DrawerPrimitive.Close>
          <Button type="button">Speichern</Button>
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  ),
};

/** Save running (`pending`): X, swipe, backdrop and footer are locked. */
export const Pending: Story = {
  render: () => (
    <SlideOver
      defaultOpen
      pending={{ pending: true, run: async () => undefined }}
    >
      <SlideOverContent>
        <SlideOverHeader>
          <SlideOverTitle>Termin bearbeiten</SlideOverTitle>
        </SlideOverHeader>
        <SlideOverBody className="text-sm text-slate-600">
          Inhalt des Panels.
        </SlideOverBody>
        <SlideOverFooter>
          <Button type="button" variant="outline">
            Abbrechen
          </Button>
          <Button type="button">Speichern</Button>
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  ),
};

/** Closing locked without a running save (X disabled). */
export const DismissDisabled: Story = {
  render: () => (
    <SlideOver defaultOpen dismissible={false}>
      <SlideOverContent>
        <SlideOverHeader close={<SlideOverCloseButton disabled />}>
          <SlideOverTitle>Personalpool</SlideOverTitle>
        </SlideOverHeader>
        <SlideOverBody className="text-sm text-slate-600">
          Eine Verschiebung wartet auf Bestätigung.
        </SlideOverBody>
      </SlideOverContent>
    </SlideOver>
  ),
};
