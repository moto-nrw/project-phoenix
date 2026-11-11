import { LockSimpleIcon } from "@phosphor-icons/react";
import type { Decorator } from "@storybook/nextjs-vite";

import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import {
  NotBookedSlotProvider,
  type NotBookedSlots,
} from "~/components/ui/not-booked-slot";
import { MODULE_LABELS, type ModuleKey } from "~/lib/modules";

/**
 * Stand-ins for the explanation `components/tenant` gives the kit (#4894):
 * the same kit pieces, without session and portal. Stories of the kit show
 * where the slot renders, not which role reads which sentence.
 */
function explanation(module: ModuleKey): string {
  return `Diese Seite gehört zum Modul ${MODULE_LABELS[module]}. Ihre Schule nutzt es nicht. Wenn Sie es brauchen, sprechen Sie mit Ihrer Leitung.`;
}

export const STORY_NOT_BOOKED_SLOTS: NotBookedSlots = {
  Alert: ({ module, className }) => (
    <Alert type="info" message={explanation(module)} className={className} />
  ),
  State: ({ module }) => (
    <EmptyState
      icon={<LockSimpleIcon className="h-12 w-12" aria-hidden="true" />}
      title={`${MODULE_LABELS[module]} ist nicht gebucht`}
      description={explanation(module)}
    />
  ),
};

/** Renders the story with the stand-in explanation, optionally for a page
 *  whose own module is not booked. */
export function withNotBookedSlots(
  pageModule: ModuleKey | null = null,
): Decorator {
  return (Story) => (
    <NotBookedSlotProvider slots={{ ...STORY_NOT_BOOKED_SLOTS, pageModule }}>
      <Story />
    </NotBookedSlotProvider>
  );
}
