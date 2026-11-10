import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { FormModal } from "./form-modal";
import { ModalProvider } from "./modal-context";
import {
  useFormModalPending,
  type FormModalOperation,
  type FormModalPending,
} from "./use-form-modal-pending";

// Wrapper component with portal target
function TestWrapper({ children }: { children: ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

function deferred() {
  let resolve!: () => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

// Lets a second trigger reach the running contract, as Enter in a form would.
let lastPending: FormModalPending | null = null;
let lastTask: ((op: FormModalOperation) => Promise<void>) | null = null;
function EnterSubmit() {
  return (
    <button
      type="button"
      onClick={() => {
        if (lastPending && lastTask) void lastPending.run(lastTask);
      }}
    >
      Nochmal
    </button>
  );
}

function PendingHarness({
  isOpen = true,
  onClose,
  task,
}: {
  isOpen?: boolean;
  onClose: () => void;
  task: (op: FormModalOperation) => Promise<void>;
}) {
  const save = useFormModalPending(isOpen);
  const [note, setNote] = useState("");
  lastPending = save;
  lastTask = task;
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Test"
      pending={save}
      footer={
        <>
          <button type="button" onClick={onClose}>
            Abbrechen
          </button>
          <button type="button" onClick={() => void save.run(task)}>
            Speichern
          </button>
        </>
      }
    >
      <p>Content</p>
      <label>
        Notiz
        <textarea value={note} onChange={(e) => setNote(e.target.value)} />
      </label>
    </FormModal>
  );
}

describe("FormModal", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    document.documentElement.style.overflow = "";
  });

  it("should not render when isOpen is false", async () => {
    const { unmount } = render(
      <TestWrapper>
        <FormModal isOpen={false} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );

    expect(screen.queryByText("Test Modal")).not.toBeInTheDocument();
    unmount();

    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );
    await act(async () => {
      vi.advanceTimersByTime(20);
    });
    // Anker: geöffnet findet dieselbe Abfrage den Dialog.
    expect(screen.getByText("Test Modal")).toBeInTheDocument();
  });

  it("should render when isOpen is true", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByText("Test Modal")).toBeInTheDocument();
    expect(screen.getByText("Modal content")).toBeInTheDocument();
  });

  it("suspends the dialog without unmounting its children", async () => {
    function Draft() {
      const [value, setValue] = useState("");
      return (
        <input
          aria-label="Entwurf"
          value={value}
          onChange={(event) => setValue(event.target.value)}
        />
      );
    }

    const renderModal = (suspended: boolean) => (
      <TestWrapper>
        <FormModal
          isOpen
          onClose={vi.fn()}
          title="Test Modal"
          suspended={suspended}
        >
          <Draft />
        </FormModal>
      </TestWrapper>
    );
    const { rerender } = render(renderModal(false));

    fireEvent.change(screen.getByRole("textbox", { name: "Entwurf" }), {
      target: { value: "bleibt erhalten" },
    });
    rerender(renderModal(true));

    expect(screen.queryByRole("dialog", { name: "Test Modal" })).toBeNull();
    expect(screen.getByDisplayValue("bleibt erhalten")).toBeInTheDocument();

    rerender(renderModal(false));
    expect(
      screen.getByRole("dialog", { name: "Test Modal" }),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue("bleibt erhalten")).toBeInTheDocument();
  });

  it("dims the page with the shared backdrop tint once entered", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // Same tint as Modal, Drawer and SlideOver (#2932).
    expect(
      screen.getByRole("button", { name: /hintergrund.*schließen/i }),
    ).toHaveClass("bg-black/40", "backdrop-blur-sm");
  });

  it("uses the visible title as the dialog accessible name", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const dialog = screen.getByRole("dialog", { name: "Test Modal" });
    const title = screen.getByRole("heading", { name: "Test Modal" });
    expect(dialog).toHaveAttribute("aria-labelledby", title.id);
  });

  it("should call onClose when close button is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const closeButton = screen.getAllByRole("button", {
      name: /schließen/i,
    })[0];
    fireEvent.click(closeButton!);

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("removes the backdrop from the tab order so FocusScope cannot land on it", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <input data-testid="first-field" />
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const backdrop = screen.getByRole("button", {
      name: /hintergrund.*schließen/i,
    });
    expect(backdrop).toHaveAttribute("tabIndex", "-1");
  });

  it("should call onClose when backdrop is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const backdrop = screen.getByRole("button", {
      name: /hintergrund.*schließen/i,
    });
    fireEvent.click(backdrop);

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("ignores backdrop clicks with isBackdropDismissDisabled but still closes on Escape (#3370)", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <FormModal
          isOpen={true}
          onClose={onClose}
          title="Test"
          isBackdropDismissDisabled
        >
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(
      screen.getByRole("button", { name: /hintergrund.*schließen/i }),
    );
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.keyDown(document, { key: "Escape" });
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("should call onClose when Escape key is pressed", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.keyDown(document, { key: "Escape" });

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // #5531: Ein Schließen-X für alle Overlays, der Kit-Knopf `ghost` `icon`.
  it("renders the close X as the kit ghost icon button", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const close = screen.getByRole("button", { name: "Modal schließen" });
    expect(close).toHaveClass(
      "h-8",
      "aspect-square",
      "pointer-coarse:min-h-11",
      "pointer-coarse:min-w-11",
      "hover:bg-gray-100",
    );
    expect(close).toHaveAttribute("type", "button");
    expect(close.className).not.toMatch(/scale-/);
  });

  it("blocks close icon, backdrop, Escape and footer while a save is pending", async () => {
    const onClose = vi.fn();
    const save = deferred();
    render(
      <TestWrapper>
        <PendingHarness onClose={onClose} task={() => save.promise} />
      </TestWrapper>,
    );
    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    const dialog = screen.getByRole("dialog", { name: "Test" });
    expect(dialog).toHaveAttribute("aria-busy", "true");
    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /hintergrund.*schließen/i }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    fireEvent.click(
      screen.getByRole("button", { name: /hintergrund.*schließen/i }),
    );
    fireEvent.keyDown(document, { key: "Escape" });
    // The footer actions are disabled through the fieldset; browsers send
    // disabled controls no click, happy-dom would, so the test reads the state.
    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).not.toHaveBeenCalled();
    expect(dialog).not.toHaveClass("animate-modalExit");
    expect(screen.getByText("Content")).toBeInTheDocument();
  });

  it("unlocks every path after a failed save and keeps the input", async () => {
    const onClose = vi.fn();
    const save = deferred();
    render(
      <TestWrapper>
        <PendingHarness
          onClose={onClose}
          task={async () => {
            await save.promise.catch(() => undefined);
          }}
        />
      </TestWrapper>,
    );
    await act(async () => {
      vi.advanceTimersByTime(20);
    });
    fireEvent.change(screen.getByRole("textbox", { name: "Notiz" }), {
      target: { value: "Entwurf" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
    await act(async () => {
      save.reject(new Error("offline"));
    });

    const dialog = screen.getByRole("dialog", { name: "Test" });
    expect(dialog).not.toHaveAttribute("aria-busy");
    expect(screen.getByRole("textbox", { name: "Notiz" })).toHaveValue(
      "Entwurf",
    );
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).toBeEnabled();
    fireEvent.keyDown(document, { key: "Escape" });
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("cancels a queued close when a save starts during the exit delay", async () => {
    const onClose = vi.fn();
    const save = deferred();
    render(
      <TestWrapper>
        <PendingHarness onClose={onClose} task={() => save.promise} />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    expect(document.querySelector('[role="dialog"]')).toHaveClass(
      "animate-modalExit",
    );

    // Save clicked within the 250ms exit delay: the request is now in flight
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByText("Test")).toBeInTheDocument();
    expect(document.querySelector('[role="dialog"]')).toHaveClass(
      "animate-modalEnter",
    );
  });

  it("keeps the modal open when a save queued during the exit delay fails fast", async () => {
    const onClose = vi.fn();
    const save = deferred();
    render(
      <TestWrapper>
        <PendingHarness
          onClose={onClose}
          task={async () => {
            await save.promise.catch(() => undefined);
          }}
        />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));

    // Save clicked within the 250ms exit delay ...
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await act(async () => {
      vi.advanceTimersByTime(50);
    });
    // ... and the request fails fast, lifting the lock before the queued
    // close would have fired.
    await act(async () => {
      save.reject(new Error("offline"));
    });

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByText("Test")).toBeInTheDocument();
    expect(document.querySelector('[role="dialog"]')).toHaveClass(
      "animate-modalEnter",
    );
  });

  it("clears an earlier queued close when dismissed again during the exit delay", async () => {
    const onClose = vi.fn();
    const save = deferred();
    render(
      <TestWrapper>
        <PendingHarness
          onClose={onClose}
          task={async () => {
            await save.promise.catch(() => undefined);
          }}
        />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // Dismiss twice in quick succession: without cleanup the first timer
    // would keep running untracked next to the second one.
    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    await act(async () => {
      vi.advanceTimersByTime(100);
    });
    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));

    // Save clicked within the exit delay ...
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await act(async () => {
      vi.advanceTimersByTime(50);
    });
    // ... and the request fails fast: only the tracked (newest) timer was
    // cancelled — the first dismissal's timer must not close the modal now.
    await act(async () => {
      save.reject(new Error("offline"));
    });

    await act(async () => {
      vi.advanceTimersByTime(400);
    });

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByText("Test")).toBeInTheDocument();
    expect(document.querySelector('[role="dialog"]')).toHaveClass(
      "animate-modalEnter",
    );
  });

  it("lets a late answer leave a reopened dialog alone", async () => {
    const onClose = vi.fn();
    const save = deferred();
    const answers: boolean[] = [];
    const harness = (isOpen: boolean) => (
      <TestWrapper>
        <PendingHarness
          isOpen={isOpen}
          onClose={onClose}
          task={async (op) => {
            await save.promise;
            answers.push(op.isCurrent());
            if (op.isCurrent()) onClose();
          }}
        />
      </TestWrapper>
    );
    const { rerender } = render(harness(true));
    await act(async () => {
      vi.advanceTimersByTime(20);
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(screen.getByRole("dialog", { name: "Test" })).toHaveAttribute(
      "aria-busy",
      "true",
    );

    // The parent closes and reopens the same mounted dialog meanwhile.
    rerender(harness(false));
    rerender(harness(true));
    await act(async () => {
      vi.advanceTimersByTime(20);
    });
    const reopened = screen.getByRole("dialog", { name: "Test" });
    expect(reopened).not.toHaveAttribute("aria-busy");
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeEnabled();

    await act(async () => {
      save.resolve();
    });

    expect(answers).toEqual([false]);
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Test" })).toBeInTheDocument();
  });

  it("lets a late answer after unmount close nothing", async () => {
    const onClose = vi.fn();
    const save = deferred();
    const answers: boolean[] = [];
    const { unmount } = render(
      <TestWrapper>
        <PendingHarness
          onClose={onClose}
          task={async (op) => {
            await save.promise;
            answers.push(op.isCurrent());
            if (op.isCurrent()) onClose();
          }}
        />
      </TestWrapper>,
    );
    await act(async () => {
      vi.advanceTimersByTime(20);
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(screen.getByRole("dialog", { name: "Test" })).toHaveAttribute(
      "aria-busy",
      "true",
    );

    unmount();
    await act(async () => {
      save.resolve();
    });

    expect(answers).toEqual([false]);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("runs one save at a time and closes after a current success", async () => {
    const onClose = vi.fn();
    const save = deferred();
    const task = vi.fn(async (op: FormModalOperation) => {
      await save.promise;
      if (op.isCurrent()) onClose();
    });
    render(
      <TestWrapper>
        <PendingHarness onClose={onClose} task={task} />
        <EnterSubmit />
      </TestWrapper>,
    );
    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    // A second trigger that bypasses the disabled button (Enter in a form).
    fireEvent.click(screen.getByRole("button", { name: "Nochmal" }));
    await act(async () => {
      save.resolve();
    });

    expect(task).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("should render footer when provided", async () => {
    render(
      <TestWrapper>
        <FormModal
          isOpen={true}
          onClose={vi.fn()}
          title="Test"
          footer={<button type="button">Submit</button>}
        >
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByRole("button", { name: "Submit" })).toBeInTheDocument();
  });

  it("lets a tall footer shrink the content area instead of being clipped", async () => {
    render(
      <TestWrapper>
        <FormModal
          isOpen={true}
          onClose={vi.fn()}
          title="Test"
          footer={<button type="button">Submit</button>}
        >
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const content = document.querySelector("[data-modal-content]");
    const dialog = content?.parentElement;
    const footer = dialog?.lastElementChild;

    // The dialog is capped at max-h and clips its overflow, so the header and
    // footer may not claim height the content area already reserved. Flex
    // column plus a shrinkable content area keeps the footer actions on
    // screen on short viewports (e.g. the expanded delete confirmation).
    expect(dialog).toHaveClass("flex", "flex-col", "overflow-hidden");
    expect(content).toHaveClass("min-h-0", "overflow-y-auto");
    expect(dialog?.firstElementChild).toHaveClass("shrink-0");
    expect(footer).toHaveClass("shrink-0");
  });

  it("should hide html overflow when open", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(document.documentElement.style.overflow).toBe("hidden");
  });

  it("should restore html overflow when closed", async () => {
    const { rerender } = render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    rerender(
      <TestWrapper>
        <FormModal isOpen={false} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(document.documentElement.style.overflow).toBe("");
  });

  describe("size prop", () => {
    it("should apply lg size by default", async () => {
      render(
        <TestWrapper>
          <FormModal isOpen={true} onClose={vi.fn()} title="Test">
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("max-w-2xl");
    });

    it("should apply sm size when specified", async () => {
      render(
        <TestWrapper>
          <FormModal isOpen={true} onClose={vi.fn()} title="Test" size="sm">
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("max-w-md");
    });

    it("should apply xl size when specified", async () => {
      render(
        <TestWrapper>
          <FormModal isOpen={true} onClose={vi.fn()} title="Test" size="xl">
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("max-w-4xl");
    });
  });

  describe("mobilePosition prop", () => {
    it("should use bottom position by default", async () => {
      render(
        <TestWrapper>
          <FormModal isOpen={true} onClose={vi.fn()} title="Test">
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("rounded-t-2xl");
    });

    it("should use center position when specified", async () => {
      render(
        <TestWrapper>
          <FormModal
            isOpen={true}
            onClose={vi.fn()}
            title="Test"
            mobilePosition="center"
          >
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("rounded-2xl");
    });
  });

  it("should dispatch mobile-modal-open event when opened", async () => {
    const eventSpy = vi.fn();
    globalThis.addEventListener("mobile-modal-open", eventSpy);

    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(eventSpy).toHaveBeenCalled();

    globalThis.removeEventListener("mobile-modal-open", eventSpy);
  });

  it("should dispatch mobile-modal-close event when closed", async () => {
    const eventSpy = vi.fn();
    globalThis.addEventListener("mobile-modal-close", eventSpy);

    const { rerender } = render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    rerender(
      <TestWrapper>
        <FormModal isOpen={false} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(eventSpy).toHaveBeenCalled();

    globalThis.removeEventListener("mobile-modal-close", eventSpy);
  });

  it("renders the error slot as an alert above the content", async () => {
    render(
      <TestWrapper>
        <FormModal
          isOpen={true}
          onClose={vi.fn()}
          title="Test Modal"
          error="Bitte einen Namen eintragen."
        >
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Bitte einen Namen eintragen.");
    expect(
      alert.compareDocumentPosition(screen.getByText("Modal content")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("renders no alert without an error", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // Anker: der offene Dialog zeigt seinen Inhalt.
    expect(screen.getByText("Modal content")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
