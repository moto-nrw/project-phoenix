import type { FullConfig } from "@playwright/test";
import { startHarness } from "./harness";

export default async function globalSetup(
  _config: FullConfig,
): Promise<() => Promise<void>> {
  return startHarness();
}
