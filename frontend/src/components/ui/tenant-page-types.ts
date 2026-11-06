import type { InputHTMLAttributes, ReactNode } from "react";
import type {
  ActiveFilter,
  FilterConfig,
  OverflowMenuItem,
  PageHeaderWithSearchProps,
} from "~/components/ui/page-header/types";
import type { FormErrorDetail } from "~/components/ui/form-error";

export interface TenantPageTab {
  readonly value: string;
  readonly label: string;
  /** Zahl rechts am Reiter, zum Beispiel offene Anfragen. */
  readonly badge?: number;
  readonly disabled?: boolean;
  /**
   * Zielpfad, wenn der Reiter auf eine eigene Seite führt (die Register einer
   * Sammlung: „Kinder · Meine Gruppen · Stammdaten"). Dann rendert der Reiter
   * ein echtes `<a>`, damit Mittelklick, „in neuem Tab öffnen" und die
   * Tastatur funktionieren — ein Knopf mit `router.push` kann das nicht.
   * Ohne `href` schaltet der Reiter wie bisher nur den Inhalt um.
   */
  readonly href?: string;
}

export interface TenantPageProps {
  readonly title: string;
  /**
   * Statuszeile unter dem Titel: echte Zahlen der Seite, die sie ohnehin lädt
   * („116 Kinder · 107 zuhause · 9 krank"). Kein Erklärsatz. Während des
   * Ladens rendert das Gerüst an dieser Stelle ein Skelett.
   */
  readonly stats?: ReactNode;
  /** Zeigt statt der Statuszeile ein Skelett. */
  readonly statsLoading?: boolean;
  /**
   * Erklärung der Seite, die man zum Kennenlernen braucht, aber nicht jeden
   * Tag liest. Sie liegt hinter einem Info-Knopf neben dem Titel und kostet
   * im Alltag keine Zeile (#4385). Per Tipp, Klick und Tastatur erreichbar,
   * deshalb auch für Text mit Link; ein Tooltip trägt nur reinen Text.
   */
  readonly info?: ReactNode;
  /** Aktionen rechts in der Titelzeile (Primäraktion, Export, Kebab). */
  readonly actions?: ReactNode;
  /** Element links vom Titel, zum Beispiel Avatar oder Konzept-Icon. */
  readonly leading?: ReactNode;
  /** Hebt den Titel eine Stufe an (Startseiten mit Begrüßung). */
  readonly prominent?: boolean;
  /**
   * Blendet den Rückweg über dem Titel ein (Unterseiten), auf jeder Breite.
   * Eine Seite baut keinen eigenen Zurück-Knopf in den Inhalt.
   */
  readonly back?: boolean;
  /** Ziel des Rückwegs (Pfad ohne Schulpräfix). Ohne Angabe die Datenverwaltung. */
  readonly backHref?: string;
  /** Sichtbarer Text des Rückwegs, nennt das Ziel („Zurück zu den Räumen"). */
  readonly backLabel?: string;
  /** Blendet die Kopfkarte nur für eine druckbare Inhaltsansicht aus. */
  readonly printHideHeader?: boolean;

  /** Suchfeld in der Kopfkarte. */
  readonly search?: {
    readonly value: string;
    readonly onChange: (value: string) => void;
    readonly placeholder?: string;
    /** Attribute des zugrunde liegenden Suchfelds, etwa `disabled`. */
    readonly inputProps?: InputHTMLAttributes<HTMLInputElement>;
    /** Verzögert die Übergabe der Eingabe an die Seite. */
    readonly debounceMs?: number;
    /** Setzt einen noch nicht übergebenen Suchentwurf zurück. */
    readonly resetKey?: string | number;
  };
  readonly filters?: readonly FilterConfig[];
  readonly activeFilters?: readonly ActiveFilter[];
  readonly onClearAllFilters?: () => void;
  readonly overflowMenu?: readonly OverflowMenuItem[];
  /**
   * Zaehler-Plakette in der Such- und Filterzeile, z. B. die Zahl der
   * gefundenen Kinder. Nur fuer eine Zahl, die sich mit der Filterung aendert
   * — feste Kennzahlen gehoeren in `stats`.
   */
  readonly badge?: {
    readonly icon?: ReactNode;
    readonly count: number;
    readonly label?: string;
  };
  /**
   * Sammelt viele Filter hinter einer Schaltflaeche statt sie in die Zeile zu
   * legen. Ab etwa drei Filtern ist das die richtige Wahl — eine Zeile, die
   * umbricht, ist keine Zeile mehr. Mit `filterSections` bekommt die Flaeche
   * dahinter Ueberschriften.
   */
  readonly filterVariant?: "default" | "quiet";
  readonly filterSections?: PageHeaderWithSearchProps["filterSections"];
  /** Ab welcher Breite die Filter wieder in der Zeile stehen duerfen. */
  readonly desktopFiltersFrom?: PageHeaderWithSearchProps["desktopFiltersFrom"];
  /** Aktive Filter als Zahl auf der Schaltflaeche statt als Chip-Reihe. */
  readonly activeFilterDisplay?: PageHeaderWithSearchProps["activeFilterDisplay"];
  /**
   * Hauptaktion IN der Such- und Filterzeile statt in der Titelzeile. Nur fuer
   * Aktionen, die zur Liste darunter gehoeren (An- und Abmelden).
   */
  readonly primaryAction?: ReactNode;

  /**
   * Fertige Such- und Filterzeile fuer Layout-Adapter (DatabasePageLayout),
   * die ihre PageHeaderWithSearch selbst konfigurieren. Seiten nutzen
   * search/filters und NICHT diesen Slot.
   */
  readonly searchSlot?: ReactNode;

  /**
   * Hoehe der Elemente im searchSlot. "controls" (Standard) zieht Knoepfe,
   * Felder und Auswahllisten auf die einheitliche Bedienhoehe von 36 px.
   * "natural" laesst dem Slot seine eigene Hoehe -- fuer Inhalte, die keine
   * Bedienzeile sind, sondern die Hauptaktion der Seite (die grossen
   * Aktionskarten in der Kindakte).
   */
  readonly searchSlotHeight?: "controls" | "natural";

  /** Horizontale Seitenreiter unter der Kopfkarte. */
  readonly tabs?: {
    readonly value: string;
    readonly onChange: (value: string) => void;
    readonly items: readonly TenantPageTab[];
    /** Vorlesbarer Name der Reiterleiste. */
    readonly label?: string;
  };

  /**
   * Fehlerzustand: ersetzt den Inhalt, die Kopfkarte bleibt stehen. Die
   * Objektform nimmt eine Aktion auf, damit ein „Erneut versuchen" nicht
   * ersatzlos verschwindet.
   *
   * `keepContent` stellt die Meldung ÜBER den Inhalt, statt ihn zu ersetzen.
   * Das ist der Fall, wenn über denselben Kanal auch der Fehler einer
   * Einzelaktion gemeldet wird („Kind konnte nicht hinzugefügt werden"):
   * ersetzte die Meldung dort den Inhalt, wäre die Fläche nach dem ersten
   * Fehlschlag nicht mehr bedienbar und ein zweiter Versuch unmöglich.
   * Für einen fehlgeschlagenen Erstabruf bleibt die ersetzende Form richtig —
   * dahinter steht kein Inhalt, den man stehen lassen könnte.
   */
  readonly error?:
    | string
    | { message: string; action?: ReactNode; keepContent?: boolean }
    /** Ein Ladefehler aus `useApiLoadError` (#2513): Wiederholen und
     *  Vorgangskennung kommen als Aktion mit. */
    | FormErrorDetail
    | null;
  /**
   * Ladezustand: ersetzt den Inhalt durch Skelette. `true` rendert das
   * generische Kartenskelett; ein Knoten rendert stattdessen das
   * Struktur-Skelett der Fläche (ein Plan-Raster lädt nicht wie eine
   * Kartenliste). Die vorlesbare Ankündigung und die Rolle bleiben in beiden
   * Fällen dieselben — deshalb kommt das Skelett hier herein und baut sich
   * nicht daneben eine zweite Ladefläche.
   */
  readonly loading?: boolean | ReactNode;
  /**
   * Vorlesbare Ankündigung des Ladezustands. Ohne Angabe wird sie aus dem
   * Titel gebildet („Kinder wird geladen…") — bei pluralen Titeln stimmt das
   * Verb dann nicht, deshalb kann die Seite den Satz selbst setzen.
   */
  readonly loadingLabel?: string;
  /**
   * Dialoge, Slide-overs und andere Overlays der Seite. Sie gehören NICHT zu
   * `children`: das Gerüst ersetzt den Inhalt in `loading`, `empty` und
   * `error`, und ein Dialog, der als Kind steht, wird dabei mit ausgehängt —
   * dann öffnet der Knopf im Leerzustand nichts. Was hier steht, bleibt in
   * jedem Zustand gemountet.
   */
  readonly overlays?: ReactNode;
  /**
   * Leerzustand: ersetzt den Inhalt, sobald nichts zu zeigen ist. `action`
   * wiederholt keine Kopfaktion (`actions`), sondern steht nur für eine
   * andere Handlung, etwa „Filter zurücksetzen" (#5034).
   */
  readonly empty?: {
    readonly title: string;
    readonly description?: string;
    readonly icon?: ReactNode;
    readonly action?: ReactNode;
  } | null;

  readonly children?: ReactNode;
  /** Nur für Testselektoren, nicht für Layout. */
  readonly testId?: string;
}
