import type { StaffMessagesApi } from "./staff-messages-api";

/**
 * What differs between the two portals that show the Team-Chat (#2208). The
 * OGS portal and the school portal render the SAME inbox and chat window
 * (components/messaging/team-chat-*); only the session, the proxy base path,
 * the navigation and the wording of the empty state are theirs. The bindings
 * live in lib/hooks/use-tenant-team-chat-portal.ts and
 * lib/hooks/use-school-team-chat-portal.ts.
 */
export interface TeamChatPortal {
  readonly kind: "tenant" | "school";
  readonly api: StaffMessagesApi;
  /** Cache scope prefix for SWR keys — a tenant switch must never surface the previous school's list. */
  readonly cacheScope: string;
  readonly inboxHref: string;
  readonly threadHref: (threadId: string) => string;
  readonly navigate: (href: string) => void;
  /**
   * What the portal already knows about the feature flag. The OGS portal
   * reads it from the cached tenant metadata; the school portal has no such
   * cache and passes `undefined` — the backend's stable code then decides.
   */
  readonly flagSaysEnabled: boolean | undefined;
  readonly title: string;
  readonly emptyDescription: string;
  readonly recipientHint: string;
}
