import { createLogger } from "./logger";
import {
  sanitizeHomeBlockPolicies,
  sanitizeHomeLayoutOverrides,
  type HomeBlockPolicies,
  type HomeLayoutOverrides,
} from "./home-blocks";
import { sessionFetch } from "./session-cache";

const logger = createLogger({ component: "HomeLayoutApi" });

export const HOME_LAYOUT_SWR_KEY = "home-layout";

/** Was die Startseite zum Rendern braucht: eigene Auswahl und Vorgabe der Schule. */
export interface HomeLayoutState {
  overrides: HomeLayoutOverrides;
  policies: HomeBlockPolicies;
  /** Darf diese Person die Vorgabe der Schule ändern? */
  canManagePolicies: boolean;
}

interface HomeLayoutResponse {
  data?: {
    overrides?: unknown;
    policies?: unknown;
    can_manage_policies?: unknown;
  } | null;
}

const EMPTY_STATE: HomeLayoutState = {
  overrides: {},
  policies: {},
  canManagePolicies: false,
};

/**
 * Liest Auswahl und Vorgabe in einem Zug.
 *
 * Fällt die Abfrage aus, rendert die Startseite ihren Standard statt einer
 * Fehlerseite: eine nicht gespeicherte Kachelauswahl darf niemanden von seinen
 * Zahlen aussperren.
 */
export async function fetchHomeLayout(): Promise<HomeLayoutState> {
  let response: Response;
  try {
    response = await sessionFetch("/api/settings/home-layout", {
      method: "GET",
    });
  } catch (error) {
    if (
      error instanceof Error &&
      error.message === "No authentication token available"
    ) {
      return EMPTY_STATE;
    }
    logger.error("fetch_home_layout_network_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return EMPTY_STATE;
  }

  if (response.status === 401 || response.status === 403) {
    return EMPTY_STATE;
  }

  if (!response.ok) {
    logger.error("fetch_home_layout_failed", { status: response.status });
    return EMPTY_STATE;
  }

  const result = (await response.json()) as HomeLayoutResponse;
  return {
    overrides: sanitizeHomeLayoutOverrides(result.data?.overrides),
    policies: sanitizeHomeBlockPolicies(result.data?.policies),
    canManagePolicies: result.data?.can_manage_policies === true,
  };
}

/** Speichert die eigene Auswahl. Wirft, damit der Dialog es anzeigen kann. */
export async function saveHomeLayout(
  overrides: HomeLayoutOverrides,
): Promise<void> {
  const response = await sessionFetch("/api/settings/home-layout", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ overrides }),
  });

  if (!response.ok) {
    logger.error("save_home_layout_failed", { status: response.status });
    throw new Error(
      `Die Auswahl konnte nicht gespeichert werden (${response.status})`,
    );
  }
}

/** Stellt die empfohlene Startseite wieder her. */
export async function resetHomeLayout(): Promise<void> {
  const response = await sessionFetch("/api/settings/home-layout", {
    method: "DELETE",
  });

  if (!response.ok) {
    logger.error("reset_home_layout_failed", { status: response.status });
    throw new Error(
      `Die Startseite konnte nicht zurückgesetzt werden (${response.status})`,
    );
  }
}

/** Speichert, was die Einrichtung für alle vorgibt. */
export async function saveHomeBlockPolicies(
  policies: HomeBlockPolicies,
): Promise<void> {
  const response = await sessionFetch("/api/settings/home-layout/policies", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ policies }),
  });

  if (!response.ok) {
    logger.error("save_home_block_policies_failed", {
      status: response.status,
    });
    throw new Error(
      `Die Vorgabe konnte nicht gespeichert werden (${response.status})`,
    );
  }
}
