import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Button } from "./button";
import { ChoiceModal } from "./choice-modal";
import { ConfirmDeleteModal } from "./confirm-delete-modal";
import { ExportDialog } from "./export-dialog";
import { FormModal } from "./form-modal";
import { Modal } from "./modal";
import { ModalCancelButton } from "./modal-frame";
import { MODAL_CLOSE_DELAY_MS, MODAL_MOTION_STYLE } from "./motion";
import {
  SlideOver,
  SlideOverBody,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "./slide-over";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const noop = () => undefined;

// One lock for closing in every kit dialog: `isDismissDisabled` (#6575).
describe("isDismissDisabled", () => {
  it("locks the Abbrechen of a Modal footer like the close X", () => {
    render(
      <Modal
        isOpen
        onClose={noop}
        title="Speichern"
        isDismissDisabled
        footer={<ModalCancelButton onClick={noop}>Abbrechen</ModalCancelButton>}
      >
        Inhalt
      </Modal>,
    );

    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
  });

  it("hides the Abbrechen of a stacked phone sheet below sm only", () => {
    render(
      <Modal
        isOpen
        onClose={noop}
        title="Abmelden"
        mobileSheet
        footer={<ModalCancelButton onClick={noop}>Abbrechen</ModalCancelButton>}
      >
        Inhalt
      </Modal>,
    );

    expect(screen.getByRole("button", { name: "Abbrechen" })).toHaveClass(
      "hidden",
      "sm:inline-flex",
    );
  });

  it("keeps the Abbrechen of a phone sheet whose actions stay one row", () => {
    render(
      <Modal
        isOpen
        onClose={noop}
        title="Formular"
        mobileSheet
        sheetFooter="row"
        footer={<ModalCancelButton onClick={noop}>Abbrechen</ModalCancelButton>}
      >
        Inhalt
      </Modal>,
    );

    expect(screen.getByRole("button", { name: "Abbrechen" })).not.toHaveClass(
      "hidden",
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

  it("locks FormModal, ChoiceModal and ConfirmDeleteModal without a running operation", () => {
    const { unmount } = render(
      <FormModal isOpen onClose={noop} title="Formular" isDismissDisabled>
        Inhalt
      </FormModal>,
    );
    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).toBeDisabled();
    unmount();

    const choice = render(
      <ChoiceModal
        isOpen
        onClose={noop}
        title="Termin bearbeiten"
        options={[{ value: "single", label: "Nur diesen Termin" }]}
        onSelect={noop}
        isDismissDisabled
      />,
    );
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Nur diesen Termin" }),
    ).toBeEnabled();
    choice.unmount();

    render(
      <ConfirmDeleteModal
        isOpen
        title="Gerät löschen?"
        description="Das Gerät wird entfernt."
        gate={{ mode: "twoStep" }}
        onConfirm={noop}
        onClose={noop}
        loading={false}
        error=""
        isDismissDisabled
      />,
    );
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).toBeDisabled();
  });

  it("locks the ExportDialog's X and Abbrechen without a running export", () => {
    render(
      <ExportDialog
        isOpen
        onClose={noop}
        title="Liste exportieren"
        formats={["pdf"]}
        run={{
          busy: null,
          error: "",
          bodyRef: { current: null },
          download: noop,
          updateFormat: noop,
        }}
        isDismissDisabled
      />,
    );

    expect(
      screen.getByRole("button", { name: "Export schließen" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Herunterladen" })).toBeEnabled();
  });
});

describe("motion", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  });

  it("closes a Modal only after its exit animation has run", async () => {
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
      vi.advanceTimersByTime(MODAL_CLOSE_DELAY_MS - 1);
    });
    expect(MODAL_CLOSE_DELAY_MS).toBeGreaterThan(
      Number.parseInt(MODAL_MOTION_STYLE["--moto-modal-exit"], 10),
    );
    expect(onClose).not.toHaveBeenCalled();
    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("animates the Modal with the durations from motion.ts", () => {
    render(
      <Modal isOpen onClose={noop} title="Titel">
        Inhalt
      </Modal>,
    );
    const root = screen.getByRole("dialog").parentElement;
    expect(root?.style.getPropertyValue("--moto-modal-enter")).toBe(
      MODAL_MOTION_STYLE["--moto-modal-enter"],
    );
    expect(root?.style.getPropertyValue("--moto-modal-exit")).toBe(
      MODAL_MOTION_STYLE["--moto-modal-exit"],
    );
  });
});
