import { expect, type Locator, type Page } from "@playwright/test";

/** Opens a menu and selects an action, including after a scroll closes it. */
export async function selectOverflowAction(
  page: Page,
  trigger: Locator,
  name: string,
) {
  // Scrolling to a row can close its menu before the item click. Retry the
  // opening too, rather than waiting for an item in a menu that is closed.
  await expect(async () => {
    if ((await trigger.getAttribute("aria-expanded")) !== "true") {
      await trigger.click({ timeout: 2000 });
    }
    await page
      .getByRole("menuitem", { name, exact: true })
      .click({ timeout: 2000 });
  }).toPass({ timeout: 15000 });
}
