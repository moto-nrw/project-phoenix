"use client";

import { createContext, useContext } from "react";
import type { UnreadAnnouncement } from "~/lib/hooks/use-announcements";
import type { Profile } from "~/lib/profile-helpers";
import type { RemindersResult } from "~/lib/reminders-api";
import type { SettingsSchema } from "~/lib/settings-api";
import type { SupervisionSnapshot } from "~/lib/supervision-derive";
import type { TenantSummary } from "~/lib/tenant-api";
import type { UserContextResponse } from "~/lib/user-context-types";

/**
 * Badge counts the tenant layout preloads on the server (#2973), keyed by the
 * hook that renders them. A missing key means the server did not load it
 * (permission or feature gate, or the request failed) and the hook fetches
 * as before.
 */
export interface ShellCounts {
  staffAbsencesPending?: number;
  messagesUnread?: number;
  teamChatUnread?: number;
  staffNoticesPending?: number;
  changeRequestsPending?: number;
  enrollmentRequestsPending?: number;
  careWithdrawalsPending?: number;
}

/**
 * Everything the app shell needs before it can render its navigation,
 * loaded once per full page load in the tenant layout and handed to the
 * client providers as initial values. Each field is optional: whatever the
 * server could not load, the client fetches exactly as it did before.
 */
export interface ShellBootstrap {
  /** Account the snapshot was built for. */
  accountId: string;
  userContext?: UserContextResponse;
  settingsSchema?: SettingsSchema;
  profile?: Profile;
  accountTenants?: TenantSummary[];
  reminders?: RemindersResult;
  announcements?: UnreadAnnouncement[];
  supervision?: SupervisionSnapshot;
  counts: ShellCounts;
}

const ShellSeedContext = createContext<ShellBootstrap | null>(null);

export const ShellSeedProvider = ShellSeedContext.Provider;

/** The server snapshot, or null outside the tenant layout (operator, tests). */
export function useShellSeed(): ShellBootstrap | null {
  return useContext(ShellSeedContext);
}
