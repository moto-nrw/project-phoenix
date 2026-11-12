import { screen, waitFor } from "@testing-library/react";
import { expect, vi } from "vitest";

import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { reloadDocument } from "~/lib/reload-document";

/** Forgets earlier page loads; call before the write under test. */
export function resetPageReloads(): void {
  vi.mocked(reloadDocument).mockClear();
}

/**
 * A refused reload after a write loads the page anew (#6801): the page's
 * first load then shows the refusal, without the refused data. The DOM
 * environment cannot navigate, so this waits for the call and checks that
 * no "saved" text claims the data were only stale.
 */
export async function expectPageReloadedAfterRefusal(): Promise<void> {
  await waitFor(() => expect(reloadDocument).toHaveBeenCalledTimes(1));
  expect(
    screen.queryByText(SAVED_BUT_STALE_SUFFIX, { exact: false }),
  ).not.toBeInTheDocument();
}
