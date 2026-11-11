import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, renderHook } from "@testing-library/react";

import { releaseFakeTimers } from "~/test/clock";

import { CoachMark } from "./coach-mark";
import { Modal } from "./modal";
import {
  isInsideModal,
  MODAL_SCOPE_PROPS,
  useIsModalOpen,
  useOverlay,
} from "./overlay-stack";
import { SlideOver, SlideOverContent, SlideOverTitle } from "./slide-over";

describe("overlay stack", () => {
  it("names the overlay opened last on the same level as topmost", () => {
    const lower = renderHook(() => useOverlay(true, "modal"));
    const upper = renderHook(() => useOverlay(true, "modal"));

    expect(lower.result.current.isTopmost()).toBe(false);
    expect(upper.result.current.isTopmost()).toBe(true);

    upper.unmount();
    expect(lower.result.current.isTopmost()).toBe(true);
  });

  it("orders overlays by their level before their opening order", () => {
    const coach = renderHook(() => useOverlay(true, "coach"));
    const modal = renderHook(() => useOverlay(true, "modal"));
    const sheet = renderHook(() => useOverlay(true, "sheet"));

    expect(coach.result.current.isTopmost()).toBe(true);
    expect(modal.result.current.isTopmost()).toBe(false);
    expect(sheet.result.current.isTopmost()).toBe(false);
  });

  it("leaves an inactive overlay out of the stack", () => {
    const open = renderHook(() => useOverlay(true, "sheet"));
    const closed = renderHook(() => useOverlay(false, "modal"));

    expect(open.result.current.isTopmost()).toBe(true);
    expect(closed.result.current.isTopmost()).toBe(false);
  });

  it("reports an open dialog, but not a side panel", () => {
    const watcher = renderHook(() => useIsModalOpen());
    expect(watcher.result.current).toBe(false);

    const sheet = renderHook(() => useOverlay(true, "sheet"));
    expect(watcher.result.current).toBe(false);

    const modal = renderHook(() => useOverlay(true, "modal"));
    expect(watcher.result.current).toBe(true);

    act(() => modal.unmount());
    expect(watcher.result.current).toBe(false);
    sheet.unmount();
  });

  it("locks the page scroll only for an overlay that asks for it", () => {
    document.documentElement.style.overflow = "";

    const sheet = renderHook(() => useOverlay(true, "sheet"));
    expect(document.documentElement.style.overflow).toBe("");

    const modal = renderHook(() =>
      useOverlay(true, "modal", { lockScroll: true }),
    );
    expect(document.documentElement.style.overflow).toBe("hidden");

    modal.unmount();
    expect(document.documentElement.style.overflow).toBe("");
    sheet.unmount();
  });

  it("tells whether an element lies in a dialog", () => {
    const { getByText } = render(
      <div>
        <div {...MODAL_SCOPE_PROPS}>
          <span>innen</span>
        </div>
        <span>außen</span>
      </div>,
    );

    expect(isInsideModal(getByText("innen"))).toBe(true);
    expect(isInsideModal(getByText("außen"))).toBe(false);
    expect(isInsideModal(null)).toBe(false);
  });
});

describe("Escape with several overlays open", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    releaseFakeTimers();
  });

  it("ends only the tour when a tour and a dialog are open", () => {
    const closeModal = vi.fn();
    const closeTour = vi.fn();
    render(
      <>
        <Modal isOpen onClose={closeModal} title="Gruppe anlegen">
          Formular
        </Modal>
        <CoachMark
          target={null}
          title="Gruppe anlegen"
          text="Hier legen Sie die Gruppe an."
          onClose={closeTour}
        />
      </>,
    );

    fireEvent.keyDown(document, { key: "Escape" });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(closeTour).toHaveBeenCalledTimes(1);
    expect(closeModal).not.toHaveBeenCalled();
  });

  it("closes the dialog once the tour has ended", () => {
    const closeModal = vi.fn();
    const { rerender } = render(
      <>
        <Modal isOpen onClose={closeModal} title="Gruppe anlegen">
          Formular
        </Modal>
        <CoachMark
          target={null}
          title="Gruppe anlegen"
          text="Hier legen Sie die Gruppe an."
          onClose={() => {}}
        />
      </>,
    );
    rerender(
      <Modal isOpen onClose={closeModal} title="Gruppe anlegen">
        Formular
      </Modal>,
    );

    fireEvent.keyDown(document, { key: "Escape" });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(closeModal).toHaveBeenCalledTimes(1);
  });

  it("keeps a side panel open while the tour above it ends", () => {
    const closePanel = vi.fn();
    const closeTour = vi.fn();
    render(
      <>
        <SlideOver open onOpenChange={closePanel}>
          <SlideOverContent aria-describedby={undefined}>
            <SlideOverTitle>Mensa</SlideOverTitle>
          </SlideOverContent>
        </SlideOver>
        <CoachMark
          target={null}
          title="Mensa"
          text="Hier sehen Sie die Mensa."
          onClose={closeTour}
        />
      </>,
    );

    fireEvent.keyDown(document.activeElement ?? document, { key: "Escape" });

    expect(closeTour).toHaveBeenCalledTimes(1);
    expect(closePanel).not.toHaveBeenCalled();
  });
});
