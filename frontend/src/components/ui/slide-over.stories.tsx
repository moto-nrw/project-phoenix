import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Drawer as DrawerPrimitive } from "vaul";

import { Button } from "~/components/ui/button";
import {
  SlideOver,
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
        <SlideOverHeader className="flex flex-row items-start justify-between">
          <div className="flex flex-col gap-1">
            <SlideOverTitle>Mensa</SlideOverTitle>
            <SlideOverDescription>Mittwoch, 24.09.2026</SlideOverDescription>
          </div>
          <SlideOverCloseButton />
        </SlideOverHeader>
        <div className="flex-1 overflow-y-auto px-5 py-4 text-sm text-slate-600">
          Inhalt des Panels.
        </div>
        <SlideOverFooter>
          <DrawerPrimitive.Close asChild>
            <Button type="button" variant="secondary">
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
      <SlideOverContent widthClass="sm:w-[640px]">
        <SlideOverHeader className="flex flex-row items-start justify-between">
          <div className="flex flex-col gap-1">
            <SlideOverTitle>Details</SlideOverTitle>
            <SlideOverDescription>
              Breiteres Panel für mehr Inhalt.
            </SlideOverDescription>
          </div>
          <SlideOverCloseButton />
        </SlideOverHeader>
        <div className="flex-1 overflow-y-auto px-5 py-4 text-sm text-slate-600">
          Inhalt des breiteren Panels.
        </div>
      </SlideOverContent>
    </SlideOver>
  ),
};
