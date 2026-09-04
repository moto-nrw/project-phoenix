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
/** Ein Seitenwechsel über die Seitenleiste (#2828). */
export interface NavigationHop {
  /** Zielpfad relativ zum Tenant-Host. */
  path: string;
  /** Beschriftung des Seitenleisten-Links. */
  linkName: string;
  /** Gruppe, die aufgeklappt werden muss, falls der Link zugeklappt ist. */
  group?: string;
  ready: (page: Page) => Locator;
}

/**
 * Der gemessene Rundgang durch die Seitenleiste. Bewusst gemischt: Kinder,
 * Räume und die Startseite haben eine eigene, seitenförmige Ladehülle,
 * Einstellungen und Statistik haben keine — genau dort zeigte sich vor #2828
 * der allgemeine Ladekringel.
 */
export const NAVIGATION_HOPS: readonly NavigationHop[] = [
  {
    path: "/dashboard",
    linkName: "Home",
    ready: (page) => page.getByText("Kinder anwesend").first(),
  },
  {
    path: "/students/search",
    linkName: "Alle Kinder",
    ready: (page) =>
      page.getByRole("button", { name: /Tippen für mehr Infos/ }).first(),
  },
  {
    path: "/rooms",
    linkName: "Räume",
    ready: (page) => page.getByText(/Kapazität:/).first(),
  },
  {
    path: "/settings",
    linkName: "Einstellungen",
    ready: (page) => page.getByRole("tab", { name: "Betrieb" }).first(),
  },
  {
    path: "/statistics",
    linkName: "Statistik",
    ready: (page) =>
      page.getByRole("columnheader", { name: /^Gruppe/ }).first(),
  },
  {
    path: "/dashboard",
    linkName: "Home",
    ready: (page) => page.getByText("Kinder anwesend").first(),
  },
];

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
