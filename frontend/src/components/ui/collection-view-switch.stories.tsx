import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { CollectionViewSwitch } from "./collection-view-switch";

const meta = {
  title: "components/ui/CollectionViewSwitch",
  component: CollectionViewSwitch,
  args: { onChange: fn() },
} satisfies Meta<typeof CollectionViewSwitch>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Kacheln aktiv; auf dem Telefon ein Symbolknopf für die Liste. */
export const Tiles: Story = { args: { value: "tiles" } };

/** Liste aktiv; auf dem Telefon ein Symbolknopf für die Kacheln. */
export const Table: Story = { args: { value: "table" } };
