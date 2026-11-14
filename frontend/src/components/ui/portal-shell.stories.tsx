import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PortalShell } from "./portal-shell";
import { SectionCard } from "./section-card";

function Demo() {
  return (
    <PortalShell
      header={
        <div className="border-b border-gray-200 bg-white px-6 py-4 text-sm font-semibold text-gray-900">
          Kopfzeile
        </div>
      }
      sidebar={
        <aside className="hidden w-60 shrink-0 border-r border-gray-200 bg-white p-4 text-sm text-gray-700 lg:block">
          Seitenleiste
        </aside>
      }
      bottomNav={null}
    >
      <SectionCard title="Inhalt">
        <p className="text-sm text-gray-600">Inhalt der Seite</p>
      </SectionCard>
    </PortalShell>
  );
}

const meta = {
  title: "components/ui/PortalShell",
  component: Demo,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Demo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
