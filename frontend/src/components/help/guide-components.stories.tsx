import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Activity, BookOpen } from "lucide-react";

import type { GuideChapter } from "./guide-data";
import { EntryPointCard, GuideShell, HelpHeader } from "./guide-components";

const chapters: readonly GuideChapter[] = [
  {
    id: "kap-1",
    title: "Erstes Kapitel",
    description: "Beschreibung des ersten Kapitels.",
    icon: Activity,
    tone: "green",
    steps: [
      {
        id: "s1",
        title: "Schritt A",
        summary: "Ein Schritt mit Bild.",
        image: "/help/screens/mitarbeiter.webp",
        screenshot: "Mitarbeiterliste",
        steps: ["`Mitarbeiter` öffnen", "`Neu` klicken"],
      },
      {
        id: "s2",
        title: "Schritt B",
        summary: "Ein Schritt ohne Bild.",
        screenshot: "Kein Bild vorhanden",
        checklist: ["Erledigt eins", "Erledigt zwei"],
      },
    ],
  },
  {
    id: "kap-2",
    title: "Zweites Kapitel",
    description: "Beschreibung des zweiten Kapitels.",
    icon: BookOpen,
    tone: "blue",
    steps: [
      {
        id: "s3",
        title: "Schritt C",
        summary: "Ein Schritt mit Hinweis.",
        screenshot: "Ignoriert",
        callout: {
          title: "Hinweis",
          body: "Ein wichtiger Hinweis für diesen Schritt.",
        },
      },
    ],
  },
];

const meta = {
  title: "help/GuideComponents",
} satisfies Meta;

export default meta;

type EntryPointStory = StoryObj<typeof EntryPointCard>;
type HelpHeaderStory = StoryObj<typeof HelpHeader>;
type GuideShellStory = StoryObj<typeof GuideShell>;

export const EntryPoint: EntryPointStory = {
  render: () => (
    <div className="max-w-sm p-6">
      <EntryPointCard
        href="/help/setup"
        title="Ersteinrichtung"
        body="Schritt für Schritt vom leeren System zum ersten Betreuungstag."
        icon={Activity}
        points={["Zugang einrichten", "Datenverwaltung", "Go-Live"]}
      />
    </div>
  ),
};

export const HeaderWithoutPdf: HelpHeaderStory = {
  render: () => (
    <div className="p-6">
      <HelpHeader />
    </div>
  ),
};

export const HeaderWithPdf: HelpHeaderStory = {
  render: () => (
    <div className="p-6">
      <HelpHeader
        pdf={{
          href: "/help/pdfs/setup.pdf",
          download: "moto Anleitung - Ersteinrichtung.pdf",
        }}
      />
    </div>
  ),
};

export const Shell: GuideShellStory = {
  render: () => (
    <GuideShell
      eyebrow="Ersteinrichtung"
      title="Schritt für Schritt zum Start"
      description="Alle Vorbereitungen für den ersten Betreuungstag an einem Ort."
      chapters={chapters}
      activePath="ersteinrichtung"
      numbered
      note="Diese Anleitung richtet sich an Administratorinnen und Administratoren."
    />
  ),
};
