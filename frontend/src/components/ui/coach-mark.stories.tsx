import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { Button } from "./button";
import { CoachMark } from "./coach-mark";
import { SectionCard } from "./section-card";

function Spot({
  withTarget,
  action,
}: {
  readonly withTarget: boolean;
  readonly action?: string;
}) {
  const [target, setTarget] = useState<HTMLElement | null>(null);
  return (
    <div className="p-12">
      <SectionCard title="Gruppen">
        <Button type="button" variant="primary" size="md" ref={setTarget}>
          Gruppe anlegen
        </Button>
      </SectionCard>
      <CoachMark
        target={withTarget ? target : null}
        title="Gruppe anlegen"
        text="Hier legen Sie die erste Gruppe Ihrer Schule an."
        progress="Schritt 2 von 4"
        action={action}
        onNext={action ? undefined : () => {}}
        onBack={() => {}}
        onClose={() => {}}
      />
    </div>
  );
}

const meta = {
  title: "components/ui/CoachMark",
  component: Spot,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Spot>;

export default meta;
type Story = StoryObj<typeof meta>;

export const AtTarget: Story = { args: { withTarget: true } };

export const WithoutTarget: Story = { args: { withTarget: false } };

// The hand symbol carries the brand green.
export const WithAction: Story = {
  args: { withTarget: true, action: "Klicken Sie auf „Gruppe anlegen“." },
};
