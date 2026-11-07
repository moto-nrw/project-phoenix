import type { ReactNode, RefObject } from "react";
import type { FormError } from "~/components/ui/form-error";

/** Leerzustand des Posteingangs — Form wie `TenantPage.empty`. */
export interface TeamChatEmptyState {
  readonly icon?: ReactNode;
  readonly title: string;
  readonly description?: string;
}

/** Was das Seitengerüst vom Posteingang bekommt. */
export interface TeamChatInboxParts {
  readonly title: string;
  /** Statuszeile aus echten Zahlen; `null`, solange keine Liste geladen ist. */
  readonly stats: string | null;
  readonly statsLoading: boolean;
  readonly chatEnabled: boolean;
  /** Zahl der sichtbaren Unterhaltungen; `null`, solange keine Liste geladen ist. */
  readonly count: number | null;
  readonly search: {
    readonly value: string;
    readonly onChange: (value: string) => void;
    readonly placeholder: string;
  };
  readonly onlyUnread: boolean;
  readonly setOnlyUnread: (next: boolean) => void;
  readonly composeButton: ReactNode | null;
  readonly loading: boolean;
  readonly empty: TeamChatEmptyState | null;
  /** Ladefehler ohne Daten, der die Liste ersetzt. */
  readonly error: FormError | null;
  /** Fehler neben vorhandenen, möglicherweise veralteten Daten. */
  readonly staleWarning: ReactNode | null;
  readonly list: ReactNode;
  readonly overlays: ReactNode;
}

/** Rückweg zum Posteingang, unverändert durch die Hülle durchgereicht. */
export interface TeamChatThreadBack {
  readonly href: string;
  readonly label: string;
}

/** Was das Seitengerüst vom Chat-Fenster bekommt. */
export interface TeamChatThreadParts {
  readonly title: string;
  readonly previewTitle?: string;
  readonly roleLabel: string | null;
  readonly stats: string;
  readonly audience: string;
  readonly actions: ReactNode | null;
  readonly statsLoading: boolean;
  readonly state: "ready" | "disabled" | "error";
  readonly empty: {
    readonly icon: ReactNode;
    readonly title: string;
    readonly description: string;
  };
  readonly error: FormError | string | null;
  readonly loading: boolean;
  readonly back: TeamChatThreadBack;
  readonly containerRef: RefObject<HTMLDivElement | null>;
  readonly body: ReactNode;
}
