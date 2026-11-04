import { act, cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { PortalShell } from "./portal-shell";

const route = vi.hoisted(() => ({ pathname: "/ferien" }));

vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useSearchParams: () => new URLSearchParams(),
}));

function shell() {
  return (
    <PortalShell header={null} sidebar={null} bottomNav={null}>
      <div />
    </PortalShell>
  );
}

type View = ReturnType<typeof render>;

function renderShellAt(pathname: string) {
  route.pathname = pathname;
  window.history.replaceState(null, "", pathname);
  return render(shell());
}

/** Der Router bestätigt einen Link-Wechsel: Die Hülle rendert mit neuem Pfad. */
function commitRoute(view: View, pathname: string) {
  route.pathname = pathname;
  window.history.replaceState(null, "", pathname);
  view.rerender(shell());
}

function firePopState() {
  act(() => {
    window.dispatchEvent(new PopStateEvent("popstate"));
  });
}

/**
 * Zurück oder Vor im Browser, in der Reihenfolge aus dem echten Browser: erst
 * Adresse und Commit, dann `popstate`, dann stellt der Browser die Position
 * wieder her (`browserScrollY`).
 */
function traverseHistory(
  view: View,
  pathname: string,
  browserScrollY?: number,
) {
  window.history.replaceState(null, "", pathname);
  route.pathname = pathname;
  view.rerender(shell());
  firePopState();
  if (browserScrollY !== undefined) setScrollY(browserScrollY);
}

/** Die Position des Dokuments, wie sie der Browser meldet. */
function setScrollY(top: number) {
  Object.defineProperty(window, "scrollY", {
    configurable: true,
    value: top,
  });
}

function nextFrame() {
  return new Promise((resolve) => requestAnimationFrame(resolve));
}

function userScrollsTo(top: number) {
  setScrollY(top);
  act(() => {
    window.dispatchEvent(new Event("scroll"));
  });
}

describe("PortalShell: Scrollposition beim Seitenwechsel", () => {
  const scrollTo = vi.fn();
  const TO_TOP = { top: 0, behavior: "instant" };

  beforeEach(() => {
    scrollTo.mockClear();
    window.scrollTo = scrollTo;
    setScrollY(0);
  });

  afterEach(() => {
    cleanup();
  });

  it("beginnt eine neu angesteuerte Seite oben", () => {
    const view = renderShellAt("/access-log");

    commitRoute(view, "/ferien");

    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });

  it("lässt die Position beim ersten Aufruf und bei gleichem Pfad stehen", () => {
    const view = renderShellAt("/access-log");

    commitRoute(view, "/access-log");

    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("setzt nicht an den Anfang, wenn `popstate` vor dem Commit kommt", () => {
    const view = renderShellAt("/ferien");

    window.history.replaceState(null, "", "/access-log");
    firePopState();
    route.pathname = "/access-log";
    view.rerender(shell());

    expect(window.location.pathname).toBe("/access-log");
    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("holt die Position auch nach, wenn `popstate` vor dem Commit kommt", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    scrollTo.mockClear();

    window.history.replaceState(null, "", "/access-log");
    setScrollY(65);
    firePopState();

    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalledWith({ top: 1483, behavior: "instant" });
    });
    route.pathname = "/access-log";
    view.rerender(shell());
    expect(scrollTo).not.toHaveBeenCalledWith(TO_TOP);
  });

  it("holt die Position nach, wenn der Browser sie auf der kurzen Seite begrenzt hat", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    scrollTo.mockClear();

    traverseHistory(view, "/access-log", 65);

    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalledWith({ top: 1483, behavior: "instant" });
    });
  });

  it("scrollt beim Zurück nicht, wenn der Browser die Position schon erreicht hat", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(300);
    commitRoute(view, "/ferien");
    scrollTo.mockClear();

    traverseHistory(view, "/access-log", 300);
    await nextFrame();
    await nextFrame();

    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });

  it("gibt das Nachholen auf, sobald die Person selbst scrollt", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    scrollTo.mockClear();

    traverseHistory(view, "/access-log", 65);
    window.dispatchEvent(new Event("wheel"));
    await nextFrame();
    await nextFrame();

    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });

  it("setzt nach einem Zurück den nächsten Link-Wechsel wieder an den Anfang", () => {
    const view = renderShellAt("/ferien");
    traverseHistory(view, "/access-log");
    scrollTo.mockClear();

    commitRoute(view, "/abrechnung");

    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });

  it("springt bei einem Zurück innerhalb derselben Seite nicht auf eine alte Position", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    commitRoute(view, "/access-log");
    scrollTo.mockClear();
    vi.spyOn(performance, "now").mockReturnValue(performance.now() + 60_000);

    firePopState();
    await nextFrame();
    await nextFrame();
    vi.restoreAllMocks();

    expect(window.location.pathname).toBe("/access-log");
    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("zählt ein Zurück innerhalb derselben Seite nicht für den nächsten Link-Wechsel", () => {
    const view = renderShellAt("/ferien");
    firePopState();

    commitRoute(view, "/abrechnung");

    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });
});
