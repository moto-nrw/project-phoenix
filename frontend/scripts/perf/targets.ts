import { expect, type Locator, type Page } from "@playwright/test";

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
    path: "/home",
    linkName: "Startseite",
    ready: homeReady,
  },
  {
    path: "/students/search",
    linkName: "Alle Kinder",
    ready: childCardReady,
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
    group: "Verwaltung",
    ready: (page) =>
      page.getByRole("columnheader", { name: /^Gruppe/ }).first(),
  },
  {
    path: "/home",
    linkName: "Startseite",
    ready: homeReady,
  },
];

/** Ein kalter Klick in der Seitenleiste, mit oder ohne Zeigen davor (#4987). */
export interface IntentHop {
  /** Kurzname für Protokoll und Artefakte. */
  name: string;
  /** Seite, auf der der Klick beginnt. */
  from: string;
  /** Sichtbar, sobald die Ausgangsseite ihre Daten zeigt. */
  fromReady: (page: Page) => Locator;
  /** Pfad der Zielseite, ohne Query. */
  path: string;
  /**
   * Akkordeon der Seitenleiste, das offen sein muss. Gesetzt wird es wie im
   * Alltag über den gespeicherten Zustand; ein Klick auf den Bereichskopf
   * würde schon navigieren.
   */
  accordion?: "supervisions";
  /** Der Seitenleisten-Link zum Ziel. */
  link: (page: Page) => Locator;
  /** Sichtbar, sobald die Zielseite echten Inhalt zeigt; gilt erst nach `path`. */
  ready: (page: Page) => Locator;
}

/** Ein sichtbarer Seitenleisten-Link über sein Ziel; Badges ändern den Namen. */
function sidebarHref(page: Page, href: string): Locator {
  return page
    .locator(`nav a[href="${href}"], nav a[href^="${href}?"]`)
    .filter({ visible: true })
    .first();
}

/** Die Kinderkarten sind Buttons „Vorname Nachname - Details öffnen“. */
function childCardReady(page: Page): Locator {
  return page.getByRole("button", { name: / - Details öffnen$/ }).first();
}

/** Die Zeilen des Bausteins „Offene Anfragen“ kommen mit dessen Daten. */
function homeReady(page: Page): Locator {
  return page.getByText("Wünsche von Eltern").first();
}

/**
 * Die Ziele aus #4987: drei Seiten mit großem Routen-Bundle und die
 * Startseite, deren vorgeladene RSC-Antwort die Startdaten nicht enthält.
 */
export const INTENT_HOPS: readonly IntentHop[] = [
  {
    name: "kindersuche",
    path: "/students/search",
    from: "/home",
    fromReady: homeReady,
    link: (page) => sidebarHref(page, "/students/search"),
    ready: childCardReady,
  },
  {
    name: "aktuelle-aufsicht",
    path: "/active-supervisions",
    from: "/home",
    fromReady: homeReady,
    // Die Räume unter „Aktuelle Aufsicht“ sind die Links; der Bereichskopf
    // ist ein Knopf, der per `router.push` navigiert.
    accordion: "supervisions",
    link: (page) => sidebarHref(page, "/active-supervisions"),
    ready: (page) =>
      page
        .getByRole("button", {
          name: /Raum verlassen|Spontane Aktivität starten|Kinder unterwegs/,
        })
        .first(),
  },
  {
    name: "anfragen",
    path: "/anfragen",
    from: "/home",
    fromReady: homeReady,
    link: (page) => sidebarHref(page, "/anfragen"),
    // Eine Zeile der Anfragenliste: „1 offene Anfrage · Betreuungszeiten“.
    ready: (page) => page.getByText(/\d+ offene Anfragen? ·/).first(),
  },
  {
    name: "startseite",
    path: "/home",
    from: "/students/search",
    fromReady: childCardReady,
    link: (page) => sidebarHref(page, "/home"),
    ready: homeReady,
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
    // Derselbe Bereitschaftsvertrag wie bei Seitenwechsel und Rückweg.
    ready: childCardReady,
    interaction: {
      label: "4 Zeichen in die Namenssuche tippen",
      run: async (page) => {
        // Zwei Suchfelder (mobil + Desktop); nur das sichtbare tippen.
        const input = page
          .getByPlaceholder("Name suchen…")
          .filter({ visible: true })
          .first();
        await input.click();
        await input.pressSequentially("Anna", { delay: 80 });
        await expect(input).toHaveValue("Anna");
        const cards = page.getByRole("button", { name: / - Details öffnen$/ });
        await expect(cards.filter({ hasText: /Anna/i }).first()).toBeVisible();
        await expect(cards.filter({ hasNotText: /Anna/i })).toHaveCount(0);
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
        const currentWeek = page.getByRole("button", {
          name: "Diese Woche",
          exact: true,
        });
        await expect(currentWeek).toBeDisabled();
        await page.getByRole("button", { name: "Vorherige Woche" }).click();
        await expect(currentWeek).toBeEnabled();
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
        const today = page.getByRole("button", { name: "Heute", exact: true });
        await expect(today).toBeDisabled();
        await page.getByRole("button", { name: "Weiter", exact: true }).click();
        await expect(today).toBeEnabled();
        await expect(page).toHaveURL(/[?&]d=\d{4}-\d{2}-\d{2}(?:&|$)/);
      },
    },
  },
];

/** Konto, mit dem ein Ablauf gemessen wird; Zugang aus dem Seed-Profil. */
export type PerfRole = "admin" | "betreuer";

/**
 * Rückweg und erneuter Besuch (#4988): Ziel öffnen, zur Zwischenseite, Ziel
 * erneut über die Seitenleiste („warm“), wieder zur Zwischenseite, dann
 * Browser-Zurück („Rückweg“). Gemessen wird bis der Zielinhalt steht.
 */
export interface RevisitTarget {
  /** Kurzname für Protokoll und Artefakte. */
  name: string;
  role: PerfRole;
  /** Pfad der Zielseite, ohne Query. */
  path: string;
  link: (page: Page) => Locator;
  /** Sichtbar, sobald die Zielseite echten Inhalt zeigt; gilt erst nach `path`. */
  ready: (page: Page) => Locator;
  /** Zwischenseite, von der aus das Ziel erneut geöffnet wird. */
  via: {
    path: string;
    link: (page: Page) => Locator;
    ready: (page: Page) => Locator;
  };
}

/**
 * Ein Bereichskopf der Seitenleiste („Aktuelle Aufsichten“, „Gruppen“): er
 * öffnet die zuletzt gezeigte Aufsicht oder Gruppe. Die Links darunter sind
 * nach einem Seitenwechsel eingeklappt.
 */
function sidebarSection(page: Page, section: string): Locator {
  return page
    .locator(`nav [data-setup-tour="nav-section-${section}"]`)
    .filter({ visible: true })
    .first();
}

/** Die Statuszeile von „Meine Gruppe heute“ kommt mit deren Daten. */
function myGroupReady(page: Page): Locator {
  return page.getByText(/\d+ von \d+ da/).first();
}

function supervisionsSection(page: Page): Locator {
  return sidebarSection(page, "supervisions");
}

function supervisionReady(page: Page): Locator {
  return page
    .getByRole("button", {
      name: /Raum verlassen|Spontane Aktivität starten|Kinder unterwegs/,
    })
    .first();
}

const ADMIN_VIA: RevisitTarget["via"] = {
  path: "/students/search",
  link: (page) => sidebarHref(page, "/students/search"),
  ready: childCardReady,
};

/** Betreuer: Kindersuche ist nicht sicher in der Seitenleiste, Räume schon. */
const BETREUER_VIA: RevisitTarget["via"] = {
  path: "/rooms",
  link: (page) => sidebarHref(page, "/rooms"),
  ready: (page) => page.getByText(/Kapazität:/).first(),
};

export const REVISIT_TARGETS: readonly RevisitTarget[] = [
  {
    name: "aktuelle-aufsicht-admin",
    role: "admin",
    path: "/active-supervisions",
    link: supervisionsSection,
    ready: supervisionReady,
    via: ADMIN_VIA,
  },
  {
    name: "aktuelle-aufsicht-betreuer",
    role: "betreuer",
    path: "/active-supervisions",
    link: supervisionsSection,
    ready: supervisionReady,
    via: BETREUER_VIA,
  },
  {
    name: "anfragen-admin",
    role: "admin",
    path: "/anfragen",
    link: (page) => sidebarHref(page, "/anfragen"),
    ready: (page) => page.getByText(/\d+ offene Anfragen? ·/).first(),
    via: ADMIN_VIA,
  },
  {
    // Seitenleiste: der Bereichskopf „Gruppen“ öffnet die Übersicht, darunter
    // steht die eigene Gruppe.
    name: "ogs-gruppe-betreuer-seitenleiste",
    role: "betreuer",
    path: "/ogs-groups",
    link: (page) => sidebarHref(page, "/ogs-groups"),
    ready: (page) =>
      page.getByRole("region", { name: "Abwesenheiten heute" }).first(),
    via: {
      path: "/ogs-groups/overview",
      link: (page) => sidebarSection(page, "groups"),
      ready: (page) => page.getByRole("list", { name: "Gruppen" }).first(),
    },
  },
  {
    // Startseite: „Zur Gruppe“ in „Meine Gruppe heute“.
    name: "ogs-gruppe-betreuer-startseite",
    role: "betreuer",
    path: "/ogs-groups",
    link: (page) =>
      page.getByRole("link", { name: "Meine Gruppe heute: zur Gruppe" }),
    ready: (page) =>
      page.getByRole("region", { name: "Abwesenheiten heute" }).first(),
    via: {
      path: "/home",
      link: (page) => sidebarHref(page, "/home"),
      ready: myGroupReady,
    },
  },
  {
    name: "startseite-betreuer",
    role: "betreuer",
    path: "/home",
    link: (page) => sidebarHref(page, "/home"),
    ready: myGroupReady,
    via: BETREUER_VIA,
  },
];
