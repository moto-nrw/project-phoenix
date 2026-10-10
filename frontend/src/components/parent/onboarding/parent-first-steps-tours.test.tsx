import { act, fireEvent, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type {
  ParentFirstStepKey,
  ParentTourDefinition,
} from "./parent-first-steps-tours";
import { useParentFirstStepsTour } from "./parent-first-steps-tours";

let pathname = "/parents/children/1";

vi.mock("next/navigation", () => ({
  usePathname: () => pathname,
  useRouter: () => ({ push: vi.fn() }),
}));

function addVisibleTarget(id: string): HTMLElement {
  const element = document.createElement("div");
  element.id = id;
  element.getClientRects = () => [{}] as unknown as DOMRectList;
  element.getBoundingClientRect = () =>
    ({
      top: 0,
      right: 100,
      bottom: 100,
      left: 0,
      width: 100,
      height: 100,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    }) as DOMRect;
  document.body.append(element);
  return element;
}

function definitionsWithChildTour(
  childData: ParentTourDefinition,
): Readonly<Record<ParentFirstStepKey, ParentTourDefinition>> {
  const unused = { path: "/parents", stops: [] } as const;
  return {
    start: unused,
    childData,
    messagesNews: unused,
    installApp: unused,
    notifications: unused,
  };
}

describe("useParentFirstStepsTour", () => {
  it("advances only once when a navigation click reaches the skip path", async () => {
    pathname = "/";
    const settings = addVisibleTarget("settings");
    addVisibleTarget("settings-content");
    const definitions = definitionsWithChildTour({
      path: "/parents/settings",
      stops: [
        {
          targets: "#settings",
          title: "Mehr",
          text: "Mehr öffnen",
          advance: "click",
          nav: true,
          skipOnPath: "/parents/settings",
        },
        {
          targets: "#settings",
          title: "Einstellungen",
          text: "Einstellungen öffnen",
          advance: "click",
          nav: true,
          skipOnPath: "/parents/settings",
        },
        {
          targets: "#settings-content",
          title: "Benachrichtigungen",
          text: "Benachrichtigungen einstellen",
          advance: "next",
          path: "/parents/settings",
        },
      ],
    });
    const onFinished = vi.fn();
    const { result, rerender } = renderHook(() =>
      useParentFirstStepsTour(definitions, onFinished),
    );

    act(() => result.current.start("childData"));
    await waitFor(() => expect(result.current.active?.target).toBe(settings));

    fireEvent.click(settings);
    act(() => {
      pathname = "/settings";
      rerender();
    });
    await act(() => new Promise((resolve) => window.setTimeout(resolve, 300)));

    expect(result.current.active?.index).toBe(1);
  });

  it("returns to a visited tab step even after that tab became visible", async () => {
    pathname = "/parents/children/1";
    addVisibleTarget("details");
    addVisibleTarget("care-tab");
    const definitions = definitionsWithChildTour({
      path: "/parents/children",
      stops: [
        {
          targets: "#details",
          title: "Angaben",
          text: "Angaben prüfen",
          advance: "next",
        },
        {
          targets: "#care-tab",
          title: "Betreuung öffnen",
          text: "Betreuung öffnen",
          advance: "click",
          skipWhenVisible: "#care-content",
        },
        {
          targets: "#care-content",
          title: "Betreuungszeiten",
          text: "Betreuungszeiten prüfen",
          advance: "next",
        },
      ],
    });
    const onFinished = vi.fn();
    const { result } = renderHook(() =>
      useParentFirstStepsTour(definitions, onFinished),
    );

    act(() => result.current.start("childData"));
    await waitFor(() => expect(result.current.active?.index).toBe(0));

    act(() => result.current.next());
    await waitFor(() => expect(result.current.active?.index).toBe(1));

    addVisibleTarget("care-content");
    act(() => result.current.next());
    await waitFor(() => expect(result.current.active?.index).toBe(2));

    act(() => result.current.back());

    await waitFor(() => expect(result.current.active?.index).toBe(1));
  });

  it("marks an unavailable optional target as missing without waiting", async () => {
    const unavailable = document.createElement("span");
    unavailable.id = "device-unavailable";
    unavailable.hidden = true;
    document.body.append(unavailable);
    const definitions = definitionsWithChildTour({
      path: "/parents/settings",
      stops: [
        {
          targets: "#device",
          title: "Benachrichtigungen erlauben",
          text: "Benachrichtigungen einschalten",
          advance: "next",
          missingWhenPresent: "#device-unavailable",
        },
      ],
    });
    const onFinished = vi.fn();
    const { result } = renderHook(() =>
      useParentFirstStepsTour(definitions, onFinished),
    );

    act(() => result.current.start("childData"));

    await waitFor(() => expect(result.current.active?.missing).toBe(true));
  });
});
