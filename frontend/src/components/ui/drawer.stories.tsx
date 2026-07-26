import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Drawer as DrawerPrimitive } from "vaul";

import { Button } from "~/components/ui/button";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";

const meta: Meta<typeof Drawer> = {
  title: "components/ui/Drawer",
  component: Drawer,
};

export default meta;
type Story = StoryObj<typeof Drawer>;

export const BottomSheet: Story = {
  render: () => (
    <Drawer>
      <DrawerPrimitive.Trigger asChild>
        <Button type="button">Drawer öffnen</Button>
      </DrawerPrimitive.Trigger>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>Raumdetails</DrawerTitle>
          <DrawerDescription>
            Informationen zum ausgewählten Raum.
          </DrawerDescription>
        </DrawerHeader>
        <div className="p-4">
          <DrawerClose asChild>
            <Button type="button" variant="secondary">
              Schließen
            </Button>
          </DrawerClose>
        </div>
      </DrawerContent>
    </Drawer>
  ),
};

export const RightSidePanel: Story = {
  render: () => (
    <Drawer direction="right">
      <DrawerPrimitive.Trigger asChild>
        <Button type="button">Seitenpanel öffnen</Button>
      </DrawerPrimitive.Trigger>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>Filter</DrawerTitle>
          <DrawerDescription>
            Seitliches Panel ohne Ziehgriff.
          </DrawerDescription>
        </DrawerHeader>
        <div className="p-4">
          <DrawerClose asChild>
            <Button type="button" variant="secondary">
              Schließen
            </Button>
          </DrawerClose>
        </div>
      </DrawerContent>
    </Drawer>
  ),
};
