import type { Locator, Page } from "@playwright/test";

interface PerfInteraction {
  label: string;
  run: (page: Page) => Promise<void>;
}

export interface PerfTarget {
  /** Dateiname der Artefakte. */
  name: string;
  /** Pfad relativ zum Tenant-Host. */
  path: string;
  /** Sichtbar, sobald die Seite echten Inhalt zeigt (kein Skeleton). */
  ready: (page: Page) => Locator;
  interaction?: PerfInteraction;
}

/**
 * Die dichtesten Screens aus #2938. Die `ready`-Locator sind Rollen/Texte der
 * gerenderten Seite, weil die Zielseiten kaum data-testid tragen.
 */
export const PERF_TARGETS: readonly PerfTarget[] = [
  {
    name: "dashboard",
    path: "/dashboard",
    ready: (page) => page.getByText("Kinder anwesend").first(),
  },
  {
    name: "active-supervisions",
    path: "/active-supervisions",
    // Erst sichtbar, wenn die Aufsichts-Daten da sind; das Skeleton trägt
    // keine dieser Schaltflächen.
    ready: (page) =>
      page
        .getByRole("button", {
          name: /Raum verlassen|Spontane Aktivität starten|Kinder unterwegs/,
        })
        .first(),
  },
  {
    name: "students-search",
    path: "/students/search",
    // Die Kinderkarten sind Buttons „Vorname Nachname - Tippen für mehr Infos“.
    ready: (page) =>
      page.getByRole("button", { name: /Tippen für mehr Infos/ }).first(),
    interaction: {
      label: "4 Zeichen in die Namenssuche tippen",
      run: async (page) => {
        // Zwei Suchfelder (mobil + Desktop); nur das sichtbare tippen.
        const input = page
          .getByPlaceholder("Name suchen...")
          .filter({ visible: true })
          .first();
        await input.click();
        await input.pressSequentially("Anna", { delay: 80 });
      },
    },
  },
  {
    name: "ogs-groups",
    path: "/ogs-groups",
    ready: (page) =>
      // Die Abwesenheits-Region erscheint erst mit geladenen Gruppendaten.
      page.getByRole("region", { name: "Abwesenheiten heute" }).first(),
  },
  {
    name: "time-tracking",
    path: "/time-tracking",
    ready: (page) => page.getByText("Wochenübersicht").first(),
    interaction: {
      label: "Vorherige Woche in der Wochenübersicht",
      run: async (page) => {
        await page.getByRole("button", { name: "Vorherige Woche" }).click();
      },
    },
  },
  {
    name: "betreuungsplan",
    path: "/betreuungsplan",
    ready: (page) => page.getByText(/\d+\s*P\./).first(),
    interaction: {
      label: "Eine Woche weiter",
      run: async (page) => {
        await page.getByRole("button", { name: "Weiter", exact: true }).click();
      },
    },
  },
];
