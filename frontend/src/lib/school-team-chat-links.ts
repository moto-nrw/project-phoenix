/**
 * Deep-links in Team-Chat notifications are written for the OGS portal
 * (`/team-chat/{id}`, see modules/communication/internal/staffmessages/notify.go). The same
 * notification reaches a Lehrkraft's school-portal tab over /school-sse, where
 * that path does not exist (#2208). This maps it onto the school inbox before
 * the toast is shown; every other link is passed through unchanged.
 */
export const SCHOOL_MESSAGES_ROUTE = "/school/nachrichten";

export function schoolTeamChatDeepLink(deepLink: string): string {
  if (deepLink === "/team-chat") return SCHOOL_MESSAGES_ROUTE;
  if (deepLink.startsWith("/team-chat/")) {
    return `${SCHOOL_MESSAGES_ROUTE}/${deepLink.slice("/team-chat/".length)}`;
  }
  return deepLink;
}
