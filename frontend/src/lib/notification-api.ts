import { apiErrorFromResponse, transportFetch } from "./api-error";

const TEST_NOTIFICATION_URL = "/api/notifications/test";
const SCHOOL_TEST_NOTIFICATION_URL = "/api/school/notifications/test";

/**
 * Sends a fixed test notification to the logged-in account. The school
 * portal (#2208) reaches the same handler through its own session.
 *
 * Throws an ApiError (#2517): a school with notifications switched off
 * answers communication.notifications_disabled, a request that never reached
 * the API general.unavailable. The card shows the catalog text.
 */
export async function sendTestNotification(
  portal: "tenant" | "school" = "tenant",
): Promise<void> {
  const response = await transportFetch(
    portal === "school" ? SCHOOL_TEST_NOTIFICATION_URL : TEST_NOTIFICATION_URL,
    {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({}),
    },
  );

  if (response.ok) return;

  throw await apiErrorFromResponse(
    response,
    `Test notification failed (${response.status})`,
  );
}
