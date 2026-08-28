const TEST_NOTIFICATION_URL = "/api/notifications/test";
const SCHOOL_TEST_NOTIFICATION_URL = "/api/school/notifications/test";

const TEST_NOTIFICATION_DISABLED_MESSAGE =
  "Ihre Schule hat Benachrichtigungen derzeit deaktiviert.";
const TEST_NOTIFICATION_ERROR_MESSAGE =
  "Testbenachrichtigung konnte nicht gesendet werden. Prüfen Sie die Verbindung und versuchen Sie es erneut.";

/**
 * Sends a fixed test notification to the logged-in account. The school
 * portal (#2208) reaches the same handler through its own session.
 */
export async function sendTestNotification(
  portal: "tenant" | "school" = "tenant",
): Promise<void> {
  let response: Response;
  try {
    response = await fetch(
      portal === "school"
        ? SCHOOL_TEST_NOTIFICATION_URL
        : TEST_NOTIFICATION_URL,
      {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      },
    );
  } catch {
    throw new Error(TEST_NOTIFICATION_ERROR_MESSAGE);
  }

  if (response.ok) return;

  throw new Error(
    response.status === 409
      ? TEST_NOTIFICATION_DISABLED_MESSAGE
      : TEST_NOTIFICATION_ERROR_MESSAGE,
  );
}
