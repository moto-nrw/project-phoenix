import type { ParentFirstStepKey } from "./parent-first-steps-tours";

const STORAGE_VERSION = 1;

export interface ParentFirstStepsState {
  readonly completed: readonly ParentFirstStepKey[];
  readonly dismissed: boolean;
  readonly finished: boolean;
}

const EMPTY_STATE: ParentFirstStepsState = {
  completed: [],
  dismissed: false,
  finished: false,
};

export function parentFirstStepsStorageKey(accountId: string): string {
  return `parent-first-steps:v${STORAGE_VERSION}:${accountId}`;
}

export function readParentFirstStepsState(
  accountId: string,
): ParentFirstStepsState {
  try {
    const raw = localStorage.getItem(parentFirstStepsStorageKey(accountId));
    if (!raw) return EMPTY_STATE;
    const parsed = JSON.parse(raw) as Partial<ParentFirstStepsState>;
    const completed = Array.isArray(parsed.completed)
      ? parsed.completed.filter(isStepKey)
      : [];
    return {
      completed: Array.from(new Set(completed)),
      dismissed: parsed.dismissed === true,
      finished: parsed.finished === true,
    };
  } catch {
    return EMPTY_STATE;
  }
}

export function writeParentFirstStepsState(
  accountId: string,
  state: ParentFirstStepsState,
): void {
  try {
    localStorage.setItem(
      parentFirstStepsStorageKey(accountId),
      JSON.stringify(state),
    );
  } catch {
    // Die Tour bleibt in dieser Sitzung nutzbar, auch ohne Browser-Speicher.
  }
}

export function completeParentFirstStep(
  state: ParentFirstStepsState,
  step: ParentFirstStepKey,
): ParentFirstStepsState {
  if (state.completed.includes(step)) return state;
  return { ...state, completed: [...state.completed, step] };
}

function isStepKey(value: unknown): value is ParentFirstStepKey {
  return (
    value === "start" ||
    value === "childData" ||
    value === "messagesNews" ||
    value === "installApp" ||
    value === "notifications"
  );
}
