import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Plus } from "lucide-react";

import { Button, ButtonLink } from "./button";
import { SectionCard } from "./section-card";
import { TenantPage } from "./tenant-page";

const meta = {
  title: "components/ui/TenantPage",
  component: TenantPage,
} satisfies Meta<typeof TenantPage>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * Kopfkarte mit Link und Knopf nebeneinander: Die Kopfkarte setzt beide auf
 * ihre Bedienhöhe, 40 px, unter `sm` 44 px (#5531).
 */
export const HeadWithLinkAndButton: Story = {
  args: {
    title: "Kinder",
    stats: "116 Kinder · 107 zuhause · 9 krank",
    actions: (
      <>
        <ButtonLink href="/students/import" variant="outline" size="md">
          Importieren
        </ButtonLink>
        <Button type="button" size="md" className="gap-1.5">
          <Plus className="h-4 w-4" aria-hidden />
          Kind anlegen
        </Button>
      </>
    ),
    children: (
      <SectionCard title="Liste">
        <p className="text-sm text-gray-600">Inhalt der Seite</p>
      </SectionCard>
    ),
  },
};
