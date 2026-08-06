const TEST_NOTIFICATION_URL = "/api/notifications/test";

const TEST_NOTIFICATION_DISABLED_MESSAGE =
  "Ihre Schule hat Benachrichtigungen derzeit deaktiviert.";
const TEST_NOTIFICATION_ERROR_MESSAGE =
  "Testbenachrichtigung konnte nicht gesendet werden. Prüfen Sie die Verbindung und versuchen Sie es erneut.";

/** Sends a fixed test notification to the logged-in staff account. */
export async function sendTestNotification(): Promise<void> {
  let response: Response;
  try {
    response = await fetch(TEST_NOTIFICATION_URL, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({}),
    });
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
