import { waitFor as waitForDOM } from "@testing-library/react";

/**
 * Testing Library's `waitFor` for state that changes without a DOM mutation,
 * such as a `renderHook` result or a mock's calls. `waitFor` rechecks such
 * state only on its polling interval (50 ms); this one polls the same
 * assertion every 5 ms. Timeout, real timers and the assertion stay
 * Testing Library's.
 */
export const waitFor: typeof waitForDOM = (callback, options) =>
  waitForDOM(callback, { interval: 5, ...options });

/**
 * The same `waitFor` for files that also wait on the DOM: call it as
 * `stateChange.waitFor(…)` next to Testing Library's own `waitFor`. Both
 * names stay visible to the absence-assertion lint rules.
 */
export const stateChange = { waitFor };
