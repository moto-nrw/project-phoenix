import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { releaseFakeTimers } from "~/test/clock";
import { Button } from "./button";
import { Modal, ModalCancelButton } from "./modal";
import { MODAL_EXIT_MS, TOAST_FADE_CLASS, TOAST_FADE_MS } from "./motion";
import {
  SlideOver,
  SlideOverBody,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "./slide-over";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

// One lock for closing in every kit dialog: `isDismissDisabled` (#6575).
describe("isDismissDisabled", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  });
  afterEach(() => {
    releaseFakeTimers();
  });

  it("locks the Abbrechen of a Modal footer like the close X", () => {
    render(
      <Modal
        isOpen
        onClose={vi.fn()}
        title="Speichern"
        isDismissDisabled
        footer={
          <ModalCancelButton onClick={vi.fn()}>Abbrechen</ModalCancelButton>
        }
      >
        Inhalt
      </Modal>,
    );

    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
  });

  it("hides the Abbrechen of a phone sheet below sm and keeps it from sm on", () => {
    render(
      <Modal
        isOpen
        onClose={vi.fn()}
        title="Abmelden"
        mobileSheet
        footer={
          <ModalCancelButton onClick={vi.fn()}>Abbrechen</ModalCancelButton>
        }
      >
        Inhalt
      </Modal>,
    );

    expect(screen.getByRole("button", { name: "Abbrechen" })).toHaveClass(
      "hidden",
      "sm:inline-flex",
    );
  });

  it("locks every way to close a SlideOver but not its footer", () => {
    const onOpenChange = vi.fn();
    const onSave = vi.fn();
    render(
      <SlideOver open onOpenChange={onOpenChange} isDismissDisabled>
        <SlideOverContent>
          <SlideOverHeader>
            <SlideOverTitle>Personalpool</SlideOverTitle>
          </SlideOverHeader>
          <SlideOverBody>Inhalt</SlideOverBody>
          <SlideOverFooter>
            <Button type="button" onClick={onSave}>
              Speichern
            </Button>
          </SlideOverFooter>
        </SlideOverContent>
      </SlideOver>,
    );

    const close = screen.getByRole("button", { name: "Schließen" });
    expect(close).toBeDisabled();
    expect(close.closest("[data-dismissible]")).toHaveAttribute(
      "data-dismissible",
      "false",
    );
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onOpenChange).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(onSave).toHaveBeenCalledTimes(1);
  });
});

describe("motion", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  });
  afterEach(() => {
    releaseFakeTimers();
  });

  it("closes a Modal once its exit animation has run, not before", async () => {
    const onClose = vi.fn();
    render(
      <Modal isOpen onClose={onClose} title="Titel">
        Inhalt
      </Modal>,
    );
    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    await act(async () => {
      vi.advanceTimersByTime(MODAL_EXIT_MS - 1);
    });
    expect(onClose).not.toHaveBeenCalled();
    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("gives the exit animation the duration the close timer waits for", () => {
    render(
      <Modal isOpen onClose={vi.fn()} title="Titel">
        Inhalt
      </Modal>,
    );
    const root = screen.getByRole("dialog").parentElement;
    expect(root?.style.getPropertyValue("--moto-modal-exit")).toBe(
      `${MODAL_EXIT_MS}ms`,
    );
  });

  it("reads the toast timer from the toast's transition class", () => {
    expect(TOAST_FADE_CLASS).toBe(`duration-${TOAST_FADE_MS}`);
    expect(TOAST_FADE_MS).toBeGreaterThan(0);
  });
});
