import { act, cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { releaseFakeTimers } from "~/test/clock";
import { expectIdleRenderBudget } from "~/test/render-budget";

import { NavigationScrollPosition } from "./navigation-scroll-position";
import { PortalShell } from "./portal-shell";

const route = vi.hoisted(() => ({ pathname: "/ferien", search: "" }));

vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useSearchParams: () => new URLSearchParams(route.search),
}));

function shell() {
  return (
    <PortalShell header={null} sidebar={null} bottomNav={null}>
      <div />
    </PortalShell>
  );
}

type View = ReturnType<typeof render>;

function setRoute(href: string) {
  const url = new URL(href, window.location.origin);
  route.pathname = url.pathname;
  route.search = url.search;
}

function renderShellAt(href: string) {
  setRoute(href);
  window.history.replaceState(null, "", href);
  return render(shell());
}

/** Der Router bestätigt einen Link-Wechsel: Die Hülle rendert mit neuem Pfad. */
function commitRoute(view: View, href: string) {
  setRoute(href);
  window.history.replaceState(null, "", href);
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
function traverseHistory(view: View, href: string, browserScrollY?: number) {
  window.history.replaceState(null, "", href);
  setRoute(href);
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
    vi.restoreAllMocks();
  });

  it("beginnt eine neu angesteuerte Seite oben", async () => {
    const view = renderShellAt("/access-log");

    commitRoute(view, "/ferien");
    await nextFrame();

    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });

  it("lässt die Position beim ersten Aufruf und bei gleichem Pfad stehen", async () => {
    const view = renderShellAt("/access-log");

    commitRoute(view, "/access-log");
    expect(scrollTo).not.toHaveBeenCalled();

    // Anker: Die Hülle reagiert auf denselben Weg, sobald der Pfad wechselt.
    commitRoute(view, "/ferien");
    await nextFrame();
    expect(scrollTo).toHaveBeenCalledOnce();
  });

  it("setzt nicht an den Anfang, wenn `popstate` vor dem Commit kommt", async () => {
    const view = renderShellAt("/ferien");

    window.history.replaceState(null, "", "/access-log");
    firePopState();
    route.pathname = "/access-log";
    view.rerender(shell());

    expect(scrollTo).not.toHaveBeenCalled();

    // Anker: Der nächste Link-Wechsel setzt wieder an den Anfang.
    commitRoute(view, "/ferien");
    await nextFrame();
    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
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
    await nextFrame();
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

  it("merkt den sichtbaren Stand, wenn das Scroll-Ereignis noch aussteht", async () => {
    const view = renderShellAt("/activities");
    // Programmgesteuertes Scrollen kann die Position schon setzen, bevor das
    // zugehörige Ereignis in der Warteschlange abgearbeitet wird.
    setScrollY(40);
    act(() => {
      document.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    commitRoute(view, "/rooms");
    scrollTo.mockClear();

    window.history.replaceState(null, "", "/activities");
    setScrollY(0);
    firePopState();
    setRoute("/activities");
    view.rerender(shell());

    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalledWith({ top: 40, behavior: "instant" });
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

    expect(scrollTo).not.toHaveBeenCalled();
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

    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("setzt nach einem Zurück den nächsten Link-Wechsel wieder an den Anfang", async () => {
    const view = renderShellAt("/ferien");
    traverseHistory(view, "/access-log");
    scrollTo.mockClear();

    commitRoute(view, "/abrechnung");
    await nextFrame();

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
    expect(scrollTo).not.toHaveBeenCalled();

    // Anker: Der nächste Link-Wechsel setzt wieder an den Anfang.
    commitRoute(view, "/abrechnung");
    await nextFrame();
    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });

  it("zählt ein Zurück innerhalb derselben Seite nicht für den nächsten Link-Wechsel", async () => {
    const view = renderShellAt("/ferien");
    firePopState();

    commitRoute(view, "/abrechnung");
    await nextFrame();

    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });

  it("überschreibt die Position eines Zwischenziels beim schnellen Zurück nicht", async () => {
    const view = renderShellAt("/activities");
    userScrollsTo(200);
    commitRoute(view, "/overview");
    await nextFrame();
    userScrollsTo(100);
    commitRoute(view, "/calendar");
    await nextFrame();
    scrollTo.mockClear();

    traverseHistory(view, "/overview", 0);
    window.history.replaceState(null, "", "/activities");
    userScrollsTo(0);
    firePopState();
    route.pathname = "/activities";
    view.rerender(shell());
    scrollTo.mockClear();

    window.history.replaceState(null, "", "/overview");
    firePopState();
    route.pathname = "/overview";
    view.rerender(shell());

    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalledWith({ top: 100, behavior: "instant" });
    });
  });

  it("überlässt eine Sprungmarke beim Zurück dem Browser", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    await nextFrame();
    scrollTo.mockClear();

    window.history.replaceState(null, "", "/access-log#details");
    route.pathname = "/access-log";
    view.rerender(shell());
    firePopState();
    setScrollY(65);
    await nextFrame();

    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("stellt bei einem zeitnahen Zurück zu einer Query nicht die Pfadposition wieder her", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    await nextFrame();
    commitRoute(view, "/access-log");
    await nextFrame();
    scrollTo.mockClear();

    window.history.replaceState(null, "", "/access-log?tab=archiv");
    setScrollY(400);
    firePopState();
    await nextFrame();

    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("holt die Position auch nach einem verspäteten `popstate`", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    await nextFrame();
    scrollTo.mockClear();

    window.history.replaceState(null, "", "/access-log");
    setRoute("/access-log");
    view.rerender(shell());
    await nextFrame();
    // Der verspätete Browser-Scroll der verlassenen Route trifft noch ein,
    // nachdem Next die Zielroute bestätigt hat, aber vor `popstate`.
    userScrollsTo(65);
    vi.spyOn(performance, "now").mockReturnValue(performance.now() + 60_000);
    firePopState();

    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalledWith({ top: 1483, behavior: "instant" });
    });
  });

  it("lässt einen Query-Verlaufseintrag bei ausstehendem Pfadwechsel in Ruhe", async () => {
    const view = renderShellAt("/access-log");
    commitRoute(view, "/ferien");
    await nextFrame();
    userScrollsTo(500);
    commitRoute(view, "/ferien?tab=archiv");
    scrollTo.mockClear();

    window.history.replaceState(null, "", "/ferien");
    route.search = "";
    setScrollY(200);
    firePopState();
    view.rerender(shell());
    await nextFrame();

    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("lässt einen Sprungmarken-Verlaufseintrag bei ausstehendem Pfadwechsel in Ruhe", async () => {
    const view = renderShellAt("/access-log");
    commitRoute(view, "/ferien");
    await nextFrame();
    userScrollsTo(500);
    window.history.replaceState(null, "", "/ferien#details");
    window.dispatchEvent(new Event("hashchange"));
    scrollTo.mockClear();

    window.history.replaceState(null, "", "/ferien");
    setScrollY(200);
    firePopState();
    await nextFrame();

    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("bricht den geplanten Reset beim Scrollen ab", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1000);
    commitRoute(view, "/ferien");
    window.dispatchEvent(new Event("wheel"));
    userScrollsTo(300);
    await nextFrame();

    expect(scrollTo).not.toHaveBeenCalledWith(TO_TOP);
  });

  it("merkt die Position nach dem Zurück, wenn die Person weiter scrollt", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(300);
    commitRoute(view, "/ferien");
    await nextFrame();

    traverseHistory(view, "/access-log", 300);
    window.dispatchEvent(new Event("wheel"));
    userScrollsTo(500);

    window.history.replaceState(null, "", "/ferien");
    firePopState();
    route.pathname = "/ferien";
    view.rerender(shell());
    window.history.replaceState(null, "", "/access-log");
    firePopState();
    route.pathname = "/access-log";
    view.rerender(shell());
    setScrollY(65);

    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalledWith({ top: 500, behavior: "instant" });
    });
  });

  it("merkt für eine abgebrochene Zwischenroute die geplante Startposition", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1000);
    commitRoute(view, "/ferien");
    commitRoute(view, "/abrechnung");
    await nextFrame();
    scrollTo.mockClear();

    traverseHistory(view, "/ferien", 65);
    await nextFrame();
    await nextFrame();

    expect(scrollTo).not.toHaveBeenCalledWith({
      top: 1000,
      behavior: "instant",
    });
  });

  it("setzt nach einem abgebrochenen Verlaufsschritt den nächsten Link-Wechsel oben an", async () => {
    const view = renderShellAt("/ferien");

    window.history.replaceState(null, "", "/access-log");
    firePopState();
    commitRoute(view, "/abrechnung");
    await nextFrame();

    expect(scrollTo).toHaveBeenCalledExactlyOnceWith(TO_TOP);
  });

  it("beendet das Nachholen bei einem Query-Wechsel", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    await nextFrame();
    scrollTo.mockClear();

    traverseHistory(view, "/access-log", 65);
    window.history.replaceState(null, "", "/access-log?tab=archiv");
    route.search = "?tab=archiv";
    firePopState();
    view.rerender(shell());
    setScrollY(65);
    await nextFrame();
    await nextFrame();

    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("beendet das Nachholen bei einem Sprungmarken-Wechsel", async () => {
    const view = renderShellAt("/access-log");
    userScrollsTo(1483);
    commitRoute(view, "/ferien");
    await nextFrame();
    scrollTo.mockClear();

    traverseHistory(view, "/access-log", 65);
    window.history.replaceState(null, "", "/access-log#details");
    window.dispatchEvent(new Event("hashchange"));
    setScrollY(65);
    await nextFrame();
    await nextFrame();

    expect(scrollTo).not.toHaveBeenCalled();
  });

  it("stellt einen gespeicherten Seitenanfang wieder her", async () => {
    const view = renderShellAt("/access-log");
    commitRoute(view, "/ferien");
    await nextFrame();
    scrollTo.mockClear();

    traverseHistory(view, "/access-log", 65);

    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalledWith(TO_TOP);
    });
  });

  it("rendert im Leerlauf nicht von selbst neu", async () => {
    vi.useFakeTimers();
    try {
      await expectIdleRenderBudget(<NavigationScrollPosition />);
    } finally {
      releaseFakeTimers();
    }
  });
});
