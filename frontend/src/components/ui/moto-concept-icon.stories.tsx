import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { MotoConceptIcon } from "./moto-concept-icon";
import { childrenConcept } from "~/lib/concepts/children";
import { parentsConcept } from "~/lib/concepts/parents";
import { staffConcept } from "~/lib/concepts/staff";

const meta = {
  title: "components/ui/MotoConceptIcon",
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const ConceptTones: Story = {
  render: () => (
    <div className="flex items-center gap-4">
      <MotoConceptIcon concept={childrenConcept} />
      <MotoConceptIcon concept={parentsConcept} />
      <MotoConceptIcon concept={staffConcept} />
    </div>
  ),
};

export const ToneOverride: Story = {
  render: () => (
    <div className="flex items-center gap-4">
      <MotoConceptIcon concept={childrenConcept} tone="neutral" />
      <MotoConceptIcon concept={staffConcept} tone="petrol" />
    </div>
  ),
};
