import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  state: {
    portal: "tenant" as string | null,
    status: "ready",
    granted: null as boolean | null,
    schoolAllows: true,
  },
  applies: true,
}));

// The dynamic chunk resolves to a marker, so the test sees whether the gate
// would load the prompt at all.
vi.mock("next/dynamic", () => ({
  default: () =>
    function LoadedPrompt() {
      return <div>Loaded consent prompt</div>;
    },
}));
vi.mock("~/lib/analytics-consent", () => ({
  analyticsConsentApplies: () => mocks.applies,
  useAnalyticsConsent: () => mocks.state,
}));

const { AnalyticsConsentPromptGate } =
  await import("./analytics-consent-prompt-gate");

function renderGate() {
  return render(
    <AnalyticsConsentPromptGate portal="tenant" copy={{} as never} />,
  );
}

describe("AnalyticsConsentPromptGate", () => {
  beforeEach(() => {
    mocks.state = {
      portal: "tenant",
      status: "ready",
      granted: null,
      schoolAllows: true,
    };
    mocks.applies = true;
  });

  it("loads the prompt for an undecided session of a school with analytics", () => {
    renderGate();
    expect(screen.getByText("Loaded consent prompt")).toBeInTheDocument();
  });

  it.each([
    { name: "decided", change: { granted: true } },
    { name: "refused", change: { granted: false } },
    { name: "loading", change: { status: "loading" } },
    { name: "another portal", change: { portal: "parent" } },
    { name: "school switched off", change: { schoolAllows: false } },
  ])("loads nothing when $name", ({ change }) => {
    mocks.state = { ...mocks.state, ...change };
    const view = renderGate();
    expect(view.container.firstChild).toBeNull();

    // Counterpart in the same render: the undecided session loads it.
    mocks.state = {
      portal: "tenant",
      status: "ready",
      granted: null,
      schoolAllows: true,
    };
    view.rerender(
      <AnalyticsConsentPromptGate portal="tenant" copy={{} as never} />,
    );
    expect(screen.getByText("Loaded consent prompt")).toBeInTheDocument();
  });

  it("loads nothing without analytics on this deployment", () => {
    mocks.applies = false;
    const view = renderGate();
    expect(view.container.firstChild).toBeNull();
  });
});
