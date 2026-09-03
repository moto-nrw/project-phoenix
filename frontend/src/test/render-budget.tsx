import { Profiler, type ReactNode } from "react";
import { act, render } from "@testing-library/react";
import { expect, vi } from "vitest";

const IDLE_MS = 5_000;
const IDLE_STEP_MS = 100;

/** Deckel für Profiler-Commits in 5 s Leerlauf (#2939). */
export const RENDER_BUDGET_MAX_COMMITS = 20;

/**
 * Render-Budget (#2939): rendert `ui` unter einem `<Profiler>`, lässt die
 * Fake-Timer 5 s Leerlauf laufen und deckelt die Commits. Eine Effekt-Schleife
 * wie die der Bottom-Nav bis #2978 (rund 2.000 Renders pro Sekunde) fällt hier
 * unabhängig von der Runner-Geschwindigkeit auf, weil nur gezählt wird.
 * Der Aufrufer schaltet die Fake-Timer davor ein und danach wieder aus.
 */
export async function expectIdleRenderBudget(ui: ReactNode) {
  let commits = 0;
  render(
    <Profiler id="render-budget" onRender={() => void commits++}>
      {ui}
    </Profiler>,
  );
  // In Schritten, weil `act` gepufferte Updates erst am Ende ausführt: ein
  // Timer, der im Update einen neuen Timer setzt, feuert sonst nie.
  for (let elapsed = 0; elapsed < IDLE_MS; elapsed += IDLE_STEP_MS) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(IDLE_STEP_MS);
    });
  }
  expect(commits).toBeGreaterThan(0);
  expect(commits).toBeLessThanOrEqual(RENDER_BUDGET_MAX_COMMITS);
}
