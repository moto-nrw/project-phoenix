import { act, screen } from "@testing-library/react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { setTestClock } from "~/test/clock";
import { getTimeBasedGreeting, useTimeBasedGreeting } from "./greeting";

function Title() {
  return <h1>{`${useTimeBasedGreeting()}, Katrin`}</h1>;
}

describe("getTimeBasedGreeting", () => {
  it.each([
    ["2026-10-02T10:15:00+02:00", "Guten Morgen"],
    ["2026-10-02T14:00:00+02:00", "Guten Tag"],
    ["2026-10-02T21:30:00+02:00", "Guten Abend"],
  ])("grüßt um %s mit %s", (instant, greeting) => {
    expect(getTimeBasedGreeting(new Date(instant))).toBe(greeting);
  });
});

describe("useTimeBasedGreeting", () => {
  let container: HTMLDivElement | null = null;

  afterEach(() => {
    container?.remove();
    container = null;
  });

  // Der Server kennt die Uhr der Person nicht. Rendert er einen Tageszeit-
  // Gruß, widerspricht ihm der Browser, sobald beide Uhren in verschiedenen
  // Tageszeiten stehen, und React verwirft die Seite (#3764).
  it("rendert auf dem Server einen Gruß ohne Tageszeit", () => {
    setTestClock("2026-10-02T21:30:00+02:00");

    expect(renderToString(<Title />)).toContain("Hallo, Katrin");
  });

  it("zeigt nach der Hydration den Gruß der Browser-Uhr ohne Hydration-Fehler", async () => {
    setTestClock("2026-10-02T21:30:00+02:00");
    container = document.createElement("div");
    container.innerHTML = renderToString(<Title />);
    document.body.append(container);
    setTestClock("2026-10-02T10:15:00+02:00");
    const onRecoverableError = vi.fn();

    await act(async () => {
      hydrateRoot(container!, <Title />, { onRecoverableError });
    });

    expect(screen.getByRole("heading")).toHaveTextContent(
      "Guten Morgen, Katrin",
    );
    expect(onRecoverableError).not.toHaveBeenCalled();
  });
});
