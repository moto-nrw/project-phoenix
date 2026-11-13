import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { BackLink } from "./back-link";
import { TenantPageHeaderSkeleton } from "./page-skeletons";

const meta = {
  title: "components/ui/TenantPageHeaderSkeleton",
  component: TenantPageHeaderSkeleton,
} satisfies Meta<typeof TenantPageHeaderSkeleton>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Ladekopf mit Titel und Statuszeile, in den Maßen von `TenantPage`. */
export const Default: Story = { args: {} };

/** Ohne Statuszeile. */
export const WithoutStats: Story = { args: { stats: false } };

/** Alle Teile: Rückweg, Element vor dem Titel, Info-Knopf und Bedienzeile. */
export const Full: Story = {
  args: {
    leading: true,
    info: true,
    controls: true,
    back: (
      <BackLink href="/rooms" label="Zurück zu den Räumen" className="mb-2" />
    ),
  },
};

/** Kopf des Elternportals (#6613). */
export const ParentPage: Story = { args: { matches: "parent-page" } };
