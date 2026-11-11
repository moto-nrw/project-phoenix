import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ConceptIconTile } from "./concept-icon-tile";
import { childrenConcept } from "~/lib/concepts/children";
import { parentsConcept } from "~/lib/concepts/parents";
import { staffConcept } from "~/lib/concepts/staff";

const meta = {
  title: "components/ui/ConceptIconTile",
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Concepts: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <ConceptIconTile concept={childrenConcept} size="section" />
      <ConceptIconTile concept={parentsConcept} size="section" tone="blue" />
      <ConceptIconTile concept={staffConcept} size="page" tone="orange" />
      <ConceptIconTile
        concept={childrenConcept}
        size="section"
        iconTone="neutral"
      />
    </div>
  ),
};
