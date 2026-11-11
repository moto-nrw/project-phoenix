import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { CoachMark } from "./coach-mark";

const meta = {
  title: "components/ui/CoachMark",
  parameters: { layout: "fullscreen" },
} satisfies Meta;

export default meta;
type Story = StoryObj;

const noop = () => undefined;

function TourStop({ action }: { readonly action?: string }) {
  const [target, setTarget] = useState<HTMLElement | null>(null);
  return (
    <div className="h-[480px] p-8">
      <span
        ref={setTarget}
        className="inline-block rounded-lg bg-white px-4 py-2 text-sm"
      >
        Kinder
      </span>
      <CoachMark
        target={target}
        title="Kinder anlegen"
        text="Hier legen Sie die Kinder Ihrer Schule an."
        progress="Schritt 2 von 4"
        action={action}
        onNext={action ? undefined : noop}
        onBack={noop}
        onClose={noop}
      />
    </div>
  );
}

export const WithNext: Story = {
  render: () => <TourStop />,
};

// The hand symbol carries the brand green.
export const WithAction: Story = {
  render: () => <TourStop action="Klicken Sie auf „Kinder“." />,
};
