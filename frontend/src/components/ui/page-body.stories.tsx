import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { withNotBookedSlots } from "~storybook/fixtures/not-booked-slots";
import { PageBody } from "./page-body";
import { SectionCard } from "./section-card";

const meta = {
  title: "components/ui/PageBody",
  component: PageBody,
  parameters: { layout: "padded" },
  args: {
    loading: false,
    loadingLabel: "Der Bereich „Kinder“ wird geladen…",
    children: (
      <SectionCard title="Liste">
        <p className="text-sm text-gray-600">Inhalt der Seite</p>
      </SectionCard>
    ),
  },
} satisfies Meta<typeof PageBody>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Content: Story = {};

export const Loading: Story = { args: { loading: true } };

export const Empty: Story = {
  args: { empty: { title: "Keine Kinder vorhanden" } },
};

export const LoadError: Story = {
  args: { error: "Die Kinder konnten nicht geladen werden." },
};

/** Fehler über dem Inhalt, der stehen bleibt. */
export const ErrorKeepingContent: Story = {
  args: {
    error: {
      message: "Die Liste ist nicht aktuell.",
      keepContent: true,
    },
  },
};

/**
 * Das Backend lehnt mit `module.not_booked` ab (#4894): statt der roten
 * Meldung steht die Erklärung in einer Karte.
 */
export const ModuleNotBooked: Story = {
  args: {
    error: {
      message: "Das Modul ist nicht gebucht.",
      notBookedModule: "dienstplan",
    },
  },
  decorators: [withNotBookedSlots()],
};
