import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { InsetPanel } from "./inset-panel";

const meta = {
  title: "components/ui/InsetPanel",
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const AllTones: Story = {
  render: () => (
    <div className="flex w-80 flex-col gap-3 text-sm text-gray-700">
      <InsetPanel tone="muted">Grau ohne Rahmen</InsetPanel>
      <InsetPanel tone="muted" framed>
        Grau mit Rahmen
      </InsetPanel>
      <InsetPanel tone="white">Weiß</InsetPanel>
      <InsetPanel tone="danger">Fehler</InsetPanel>
      <InsetPanel tone="warning">Hinweis</InsetPanel>
      <InsetPanel tone="warning" dashed>
        Hinweis gestrichelt
      </InsetPanel>
    </div>
  ),
};
