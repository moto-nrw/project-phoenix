import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("vaul", async () => {
  const React = await import("react");
  const OpenChangeContext = React.createContext<
    ((open: boolean) => void) | undefined
  >(undefined);
  const InteractOutsideContext = React.createContext<{
    current: ((event: Event) => void) | undefined;
  } | null>(null);

  return {
    Drawer: {
      Root: ({
        children,
        direction,
        onOpenChange,
        dismissible,
      }: {
        children: React.ReactNode;
        direction?: string;
        onOpenChange?: (open: boolean) => void;
        dismissible?: boolean;
      }) => {
        const interactOutsideHandler = React.useRef<
          ((event: Event) => void) | undefined
        >(undefined);
        return (
          <OpenChangeContext.Provider value={onOpenChange}>
            <InteractOutsideContext.Provider value={interactOutsideHandler}>
              <div data-direction={direction} data-dismissible={dismissible}>
                {/* Vaul schließt selbst bei Escape und Klick auf den Hintergrund
                  und meldet das über onOpenChange. Das echte Vaul lässt sich in
                  jsdom nicht fahren, prüfbar ist aber das, was uns gehört: dass
                  der Rückruf beim Aufrufer ankommt. */}
                <button
                  type="button"
                  data-testid="vaul-dismiss"
                  onClick={() => onOpenChange?.(false)}
                />
                {/* Der Hintergrund liegt neben, nicht im Inhalt. Er schließt nur,
                  wenn der von Content registrierte Radix-Handler das Ereignis
                  nicht abweist. */}
                <button
                  type="button"
                  data-testid="vaul-outside"
                  onClick={() => {
                    const event = new Event("pointerdown", {
                      cancelable: true,
                    });
                    interactOutsideHandler.current?.(event);
                    if (!event.defaultPrevented) onOpenChange?.(false);
                  }}
                />
                {/* Ein Klick in einem Kit-Modal über dem Panel: Radix meldet
                  ihn als Außenklick mit dem Modal-Element als Ziel. */}
                <button
                  type="button"
                  data-testid="vaul-outside-in-modal"
                  onClick={() => {
                    const event = new Event("pointerdown", {
                      cancelable: true,
                    });
                    Object.defineProperty(event, "target", {
                      value: document.querySelector(
                        '[data-testid="modal-option"]',
                      ),
                    });
                    interactOutsideHandler.current?.(event);
                    if (!event.defaultPrevented) onOpenChange?.(false);
                  }}
                />
                {children}
              </div>
            </InteractOutsideContext.Provider>
          </OpenChangeContext.Provider>
        );
      },
      Portal: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      Overlay: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Content: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement> & {
          onInteractOutside?: (event: Event) => void;
          onEscapeKeyDown?: (event: KeyboardEvent) => void;
        }
      >(({ onInteractOutside, onEscapeKeyDown, children, ...props }, ref) => {
        const interactOutsideHandler = React.useContext(InteractOutsideContext);
        if (interactOutsideHandler) {
          interactOutsideHandler.current = onInteractOutside;
        }
        const close = React.useContext(OpenChangeContext);
        return (
          <div
            ref={ref}
            {...props}
            onKeyDownCapture={(event) => {
              if (event.key !== "Escape") return;
              onEscapeKeyDown?.(event.nativeEvent);
              if (!event.nativeEvent.defaultPrevented) close?.(false);
            }}
          >
            {children}
          </div>
        );
      }),
      Close: React.forwardRef<
        HTMLButtonElement,
        React.ButtonHTMLAttributes<HTMLButtonElement>
      >((props, ref) => <button ref={ref} {...props} />),
      Title: React.forwardRef<
        HTMLHeadingElement,
        React.HTMLAttributes<HTMLHeadingElement>
      >(({ children, ...props }, ref) => (
        <h2 ref={ref} {...props}>
          {children ?? "Titel"}
        </h2>
      )),
      Description: React.forwardRef<
        HTMLParagraphElement,
        React.HTMLAttributes<HTMLParagraphElement>
      >((props, ref) => <p ref={ref} {...props} />),
    },
  };
});

import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "./slide-over";
import { Modal } from "./modal";
import {
  useFormModalPending,
  type FormModalOperation,
} from "./use-form-modal-pending";

function StatefulContent() {
  const [value, setValue] = useState("");
  return (
    <input
      aria-label="Entwurf"
      value={value}
      onChange={(event) => setValue(event.target.value)}
    />
  );
}

describe("SlideOver", () => {
  it.each(["select", "date-picker", "modal"])(
    "keeps the panel open when Escape belongs to a %s above it",
    (overlay) => {
      const onOpenChange = vi.fn();
      const onEscape = vi.fn();
      const { rerender } = render(
        <SlideOver open onOpenChange={onOpenChange}>
          <SlideOverContent onEscapeKeyDown={onEscape}>
            <input aria-label="Panel input" />
            {overlay === "select" && (
              <button
                role="combobox"
                aria-expanded="true"
                aria-controls="test-listbox"
              >
                Select
              </button>
            )}
            {overlay === "date-picker" && (
              <div data-date-picker-panel>Calendar</div>
            )}
            {overlay === "modal" && (
              <Modal isOpen onClose={() => {}} title="Confirmation">
                Confirmation
              </Modal>
            )}
          </SlideOverContent>
        </SlideOver>,
      );
      const input = screen.getByRole("textbox", { name: "Panel input" });
      fireEvent.keyDown(input, { key: "Escape" });
      expect(onEscape).toHaveBeenCalledOnce();
      expect(onOpenChange).not.toHaveBeenCalled();
      rerender(
        <SlideOver open onOpenChange={onOpenChange}>
          <SlideOverContent>
            <input aria-label="Panel input" />
          </SlideOverContent>
        </SlideOver>,
      );
      fireEvent.keyDown(screen.getByRole("textbox", { name: "Panel input" }), {
        key: "Escape",
      });
      expect(onOpenChange).toHaveBeenCalledWith(false);
    },
  );

  let matches = false;
  let changeListener: (() => void) | undefined;

  beforeEach(() => {
    matches = false;
    changeListener = undefined;
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({
        get matches() {
          return matches;
        },
        addEventListener: (_event: string, listener: () => void) => {
          changeListener = listener;
        },
        removeEventListener: vi.fn(),
      })),
    );
  });

  it("uses the mobile direction for a panel already open at initial render", async () => {
    matches = true;

    render(
      <SlideOver defaultOpen>
        <SlideOverContent>Inhalt</SlideOverContent>
      </SlideOver>,
    );

    await waitFor(() => {
      expect(
        screen.getByText("Inhalt").closest("[data-direction]"),
      ).toHaveAttribute("data-direction", "bottom");
    });
  });

  it("dims and blurs the page with the shared overlay backdrop", () => {
    const { container } = render(
      <SlideOver open>
        <SlideOverContent>Inhalt</SlideOverContent>
      </SlideOver>,
    );

    const overlay = container.querySelector(".fixed.inset-0");
    expect(overlay).not.toBeNull();
    // Same tint as Modal, FormModal and Drawer — no slate-tinted or lighter
    // backdrop that would dim the page in a different gray (#2932).
    expect(overlay).toHaveClass("bg-black/40", "backdrop-blur-sm");
    expect(overlay?.className).not.toContain("bg-slate-900");
  });

  it("keeps unsaved child state when the viewport changes while open", () => {
    const { rerender } = render(
      <SlideOver open>
        <SlideOverContent>
          <StatefulContent />
        </SlideOverContent>
      </SlideOver>,
    );

    fireEvent.change(screen.getByLabelText("Entwurf"), {
      target: { value: "Ungespeichert" },
    });

    act(() => {
      matches = true;
      changeListener?.();
    });

    expect(screen.getByLabelText("Entwurf")).toHaveValue("Ungespeichert");
    expect(
      screen.getByLabelText("Entwurf").closest("[data-direction]"),
    ).toHaveAttribute("data-direction", "right");

    rerender(
      <SlideOver open={false}>
        <SlideOverContent>
          <StatefulContent />
        </SlideOverContent>
      </SlideOver>,
    );

    return waitFor(() => {
      expect(
        screen.getByLabelText("Entwurf").closest("[data-direction]"),
      ).toHaveAttribute("data-direction", "bottom");
    });
  });

  it("meldet ein Schließen durch Vaul (Escape, Klick auf den Hintergrund) an den Aufrufer", () => {
    // Beim Wechsel von Modal auf Panel ist diese Zusicherung je Verbrauchsstelle
    // entfallen, weil sie dort nicht mehr prüfbar war. Sie gehört ohnehin hierher:
    // einmal für alle Panels statt einmal je Aufrufer.
    const onOpenChange = vi.fn();

    render(
      <SlideOver open onOpenChange={onOpenChange}>
        <SlideOverContent>Inhalt</SlideOverContent>
      </SlideOver>,
    );

    fireEvent.click(screen.getByTestId("vaul-dismiss"));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("schließt bei einem Klick neben das Panel, solange nichts anderes verlangt ist", () => {
    const onOpenChange = vi.fn();

    render(
      <SlideOver open onOpenChange={onOpenChange}>
        <SlideOverContent>Inhalt</SlideOverContent>
      </SlideOver>,
    );

    fireEvent.click(screen.getByTestId("vaul-outside"));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("bleibt bei einem Klick in einem Kit-Modal darüber offen (#6148)", () => {
    const onOpenChange = vi.fn();

    render(
      <>
        <SlideOver open onOpenChange={onOpenChange}>
          <SlideOverContent>
            <StatefulContent />
          </SlideOverContent>
        </SlideOver>
        <div data-modal-focus-scope="true">
          <button type="button" data-testid="modal-option">
            Bestehendes Kind verwenden
          </button>
        </div>
      </>,
    );

    fireEvent.change(screen.getByLabelText("Entwurf"), {
      target: { value: "Felix" },
    });
    fireEvent.click(screen.getByTestId("vaul-outside-in-modal"));

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Entwurf")).toHaveValue("Felix");

    // Ein Klick neben Panel und Modal schließt weiter.
    fireEvent.click(screen.getByTestId("vaul-outside"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("bleibt mit isBackdropDismissDisabled bei einem Klick daneben offen und behält die Eingabe (#3370)", () => {
    const onOpenChange = vi.fn();
    const onInteractOutside = vi.fn();

    render(
      <SlideOver open onOpenChange={onOpenChange}>
        <SlideOverContent
          isBackdropDismissDisabled
          onInteractOutside={onInteractOutside}
        >
          <StatefulContent />
        </SlideOverContent>
      </SlideOver>,
    );

    fireEvent.change(screen.getByLabelText("Entwurf"), {
      target: { value: "0171 1234567" },
    });
    fireEvent.click(screen.getByTestId("vaul-outside"));

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Entwurf")).toHaveValue("0171 1234567");
    // Ein eigener Handler des Aufrufers läuft weiterhin.
    expect(onInteractOutside).toHaveBeenCalledTimes(1);

    // Escape und X laufen nicht über onInteractOutside und schließen weiter.
    fireEvent.click(screen.getByTestId("vaul-dismiss"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("renders the body error slot as an alert above the children", () => {
    render(
      <SlideOver open>
        <SlideOverContent>
          <SlideOverBody
            error="Bitte einen Titel eintragen."
            className="space-y-4"
          >
            <p>Formularfelder</p>
          </SlideOverBody>
        </SlideOverContent>
      </SlideOver>,
    );

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Bitte einen Titel eintragen.");
    expect(
      alert.compareDocumentPosition(screen.getByText("Formularfelder")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.getByText("Formularfelder").parentElement).toHaveClass(
      "flex-1",
      "overflow-y-auto",
      "space-y-4",
    );
  });

  it("renders the body without an alert when there is no error", () => {
    render(
      <SlideOver open>
        <SlideOverContent>
          <SlideOverBody>
            <p>Formularfelder</p>
          </SlideOverBody>
        </SlideOverContent>
      </SlideOver>,
    );

    // Anker: der Body zeigt seinen Inhalt, nur ohne Fehler.
    expect(screen.getByText("Formularfelder")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  // #5579: dasselbe Schließen-X wie Modal und FormModal, der Kit-Knopf
  // `ghost` `icon` mit 44 px unter Fingerbedienung.
  it("renders the close X as the kit ghost icon button", () => {
    render(
      <SlideOver open>
        <SlideOverContent>
          <SlideOverCloseButton aria-label="Panel schließen" />
        </SlideOverContent>
      </SlideOver>,
    );

    const close = screen.getByRole("button", { name: "Panel schließen" });
    expect(close).toHaveAttribute("type", "button");
    expect(close).toHaveAttribute("data-overlay-close");
    expect(close).toHaveClass(
      "h-8",
      "aspect-square",
      "pointer-coarse:min-h-11",
      "pointer-coarse:min-w-11",
      "hover:bg-gray-100",
    );
    expect(close.className).not.toContain("rounded-full");
  });

  it("puts the close X into the header by default", () => {
    render(
      <SlideOver open>
        <SlideOverContent>
          <SlideOverHeader>
            <SlideOverTitle>Titel</SlideOverTitle>
          </SlideOverHeader>
        </SlideOverContent>
      </SlideOver>,
    );
    expect(
      screen.getByRole("button", { name: "Schließen" }),
    ).toBeInTheDocument();
  });

  it("explains the close X and takes another sentence (#6744)", () => {
    const { unmount } = render(
      <SlideOver open>
        <SlideOverContent>
          <SlideOverHeader>
            <SlideOverTitle>Titel</SlideOverTitle>
          </SlideOverHeader>
        </SlideOverContent>
      </SlideOver>,
    );
    expect(
      screen.getByRole("button", { name: "Schließen" }),
    ).toHaveAccessibleDescription("Schließt das Seitenfenster.");
    unmount();

    render(
      <SlideOver open>
        <SlideOverContent>
          <SlideOverHeader
            close={<SlideOverCloseButton tooltip="Schließt den Entwurf." />}
          >
            <SlideOverTitle>Titel</SlideOverTitle>
          </SlideOverHeader>
        </SlideOverContent>
      </SlideOver>,
    );
    expect(
      screen.getByRole("button", { name: "Schließen" }),
    ).toHaveAccessibleDescription("Schließt den Entwurf.");
  });

  describe("pending (#6593)", () => {
    function deferred() {
      let resolve!: () => void;
      let reject!: (error: unknown) => void;
      const promise = new Promise<void>((res, rej) => {
        resolve = res;
        reject = rej;
      });
      return { promise, resolve, reject };
    }

    // Lets a second trigger reach the running contract, as Enter in a form would.
    let lastRun: (() => void) | null = null;

    function PendingPanel({
      open = true,
      context,
      onOpenChange,
      task,
    }: {
      open?: boolean;
      context?: string;
      onOpenChange: (open: boolean) => void;
      task: (op: FormModalOperation) => Promise<void>;
    }) {
      const save = useFormModalPending(open, context);
      const [note, setNote] = useState("");
      lastRun = () => void save.run(task);
      return (
        <SlideOver open={open} onOpenChange={onOpenChange} pending={save}>
          <SlideOverContent data-testid="panel">
            <SlideOverHeader>
              <SlideOverTitle>Titel</SlideOverTitle>
            </SlideOverHeader>
            <SlideOverBody>
              <textarea
                aria-label="Notiz"
                value={note}
                onChange={(event) => setNote(event.target.value)}
              />
            </SlideOverBody>
            <SlideOverFooter>
              <button type="button" onClick={() => onOpenChange(false)}>
                Abbrechen
              </button>
              <button type="button" onClick={() => void save.run(task)}>
                Speichern
              </button>
            </SlideOverFooter>
          </SlideOverContent>
        </SlideOver>
      );
    }

    function expectLocked(onOpenChange: ReturnType<typeof vi.fn>) {
      const panel = screen.getByTestId("panel");
      expect(panel).toHaveAttribute("aria-busy", "true");
      expect(screen.getByRole("button", { name: "Schließen" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
      // Vaul drags (swipe on a phone) only while dismissible.
      expect(panel.closest("[data-direction]")).toHaveAttribute(
        "data-dismissible",
        "false",
      );
      fireEvent.click(screen.getByTestId("vaul-dismiss"));
      fireEvent.click(screen.getByTestId("vaul-outside"));
      fireEvent.keyDown(panel, { key: "Escape" });
      expect(onOpenChange).not.toHaveBeenCalled();
    }

    it("sperrt X, Escape, Hintergrund, Wischen und Fußleiste, solange gespeichert wird", () => {
      const onOpenChange = vi.fn();
      const save = deferred();
      render(
        <PendingPanel onOpenChange={onOpenChange} task={() => save.promise} />,
      );
      // Anker: das Panel ist gerendert und bereit zum Speichern.
      expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
      expect(screen.getByTestId("panel")).not.toHaveAttribute("aria-busy");

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      expectLocked(onOpenChange);
    });

    it("lässt einen Link in der Fußleiste nicht wegführen, solange gespeichert wird", async () => {
      const save = deferred();
      const onNavigate = vi.fn();
      function PanelWithLink() {
        const pending = useFormModalPending(true);
        return (
          <SlideOver open pending={pending}>
            <SlideOverContent>
              <SlideOverFooter>
                <a href="#weiter" onClick={onNavigate}>
                  Weiter
                </a>
                <button
                  type="button"
                  onClick={() => void pending.run(() => save.promise)}
                >
                  Speichern
                </button>
              </SlideOverFooter>
            </SlideOverContent>
          </SlideOver>
        );
      }
      render(<PanelWithLink />);
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      const blocked = fireEvent.click(
        screen.getByRole("link", { name: "Weiter" }),
      );

      expect(blocked).toBe(false);
      expect(onNavigate).not.toHaveBeenCalled();
      await act(async () => {
        save.resolve();
      });
      fireEvent.click(screen.getByRole("link", { name: "Weiter" }));
      expect(onNavigate).toHaveBeenCalledTimes(1);
    });

    it("gibt nach einem Fehler alle Wege frei und behält die Eingabe", async () => {
      const onOpenChange = vi.fn();
      const save = deferred();
      render(
        <PendingPanel
          onOpenChange={onOpenChange}
          task={() => save.promise.catch(() => undefined)}
        />,
      );
      fireEvent.change(screen.getByRole("textbox", { name: "Notiz" }), {
        target: { value: "Entwurf" },
      });
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
      // Anker: das Panel ist gesperrt, solange gespeichert wird.
      expectLocked(onOpenChange);

      await act(async () => {
        save.reject(new Error("offline"));
      });

      const panel = screen.getByTestId("panel");
      expect(panel).not.toHaveAttribute("aria-busy");
      expect(screen.getByRole("textbox", { name: "Notiz" })).toHaveValue(
        "Entwurf",
      );
      expect(screen.getByRole("button", { name: "Schließen" })).toBeEnabled();
      expect(screen.getByRole("button", { name: "Abbrechen" })).toBeEnabled();
      expect(panel.closest("[data-direction]")).toHaveAttribute(
        "data-dismissible",
        "true",
      );
      fireEvent.keyDown(panel, { key: "Escape" });
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });

    it("hält den Fokus im Panel und gibt ihn nach einem Fehler zurück", async () => {
      const save = deferred();
      render(
        <PendingPanel
          onOpenChange={vi.fn()}
          task={() => save.promise.catch(() => undefined)}
        />,
      );
      const saveButton = screen.getByRole("button", { name: "Speichern" });
      saveButton.focus();

      fireEvent.click(saveButton);
      // The disabled button would drop focus to the page; the panel keeps it.
      expect(saveButton).toBeDisabled();
      expect(screen.getByTestId("panel")).toHaveFocus();

      await act(async () => {
        save.reject(new Error("offline"));
      });
      expect(saveButton).toBeEnabled();
      expect(saveButton).toHaveFocus();
    });

    it("startet nur einen Speichervorgang und schließt nach aktuellem Erfolg", async () => {
      const onOpenChange = vi.fn();
      const save = deferred();
      const task = vi.fn(async (op: FormModalOperation) => {
        await save.promise;
        if (op.isCurrent()) onOpenChange(false);
      });
      render(<PendingPanel onOpenChange={onOpenChange} task={task} />);

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
      // A second trigger that bypasses the disabled button (Enter in a form).
      act(() => lastRun?.());
      await act(async () => {
        save.resolve();
      });

      expect(task).toHaveBeenCalledTimes(1);
      expect(onOpenChange).toHaveBeenCalledTimes(1);
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });

    it("startet während der Schließanimation nichts", () => {
      const task = vi.fn(() => Promise.resolve());
      const { rerender } = render(
        <PendingPanel onOpenChange={vi.fn()} task={task} />,
      );
      // Vaul keeps the closed panel on screen while it slides out.
      rerender(
        <PendingPanel open={false} onOpenChange={vi.fn()} task={task} />,
      );
      // Anker: das schließende Panel steht noch mit seiner Fußleiste da.
      expect(
        screen.getByRole("button", { name: "Speichern" }),
      ).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      expect(task).not.toHaveBeenCalled();
      expect(screen.getByTestId("panel")).not.toHaveAttribute("aria-busy");
    });

    it.each([
      ["geschlossen und wieder geöffnet", { context: undefined }],
      ["auf ein anderes Objekt gewechselt", { context: "b" }],
    ])(
      "lässt eine späte Antwort in Ruhe, nachdem das Panel %s wurde",
      async (_label, next) => {
        const onOpenChange = vi.fn();
        const save = deferred();
        const answers: boolean[] = [];
        const task = async (op: FormModalOperation) => {
          await save.promise;
          answers.push(op.isCurrent());
          if (op.isCurrent()) onOpenChange(false);
        };
        const { rerender } = render(
          <PendingPanel context="a" onOpenChange={onOpenChange} task={task} />,
        );
        fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
        // Anker: das Panel speichert.
        expect(screen.getByTestId("panel")).toHaveAttribute(
          "aria-busy",
          "true",
        );

        if (next.context === undefined) {
          rerender(
            <PendingPanel
              open={false}
              context="a"
              onOpenChange={onOpenChange}
              task={task}
            />,
          );
        }
        rerender(
          <PendingPanel
            context={next.context ?? "a"}
            onOpenChange={onOpenChange}
            task={task}
          />,
        );
        expect(screen.getByTestId("panel")).not.toHaveAttribute("aria-busy");
        expect(screen.getByRole("button", { name: "Abbrechen" })).toBeEnabled();

        await act(async () => {
          save.resolve();
        });

        expect(answers).toEqual([false]);
        expect(onOpenChange).not.toHaveBeenCalled();
      },
    );

    it("lässt eine späte Antwort nach dem Aushängen nichts schließen", async () => {
      const onOpenChange = vi.fn();
      const save = deferred();
      const answers: boolean[] = [];
      const { unmount } = render(
        <PendingPanel
          onOpenChange={onOpenChange}
          task={async (op) => {
            await save.promise;
            answers.push(op.isCurrent());
            if (op.isCurrent()) onOpenChange(false);
          }}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      unmount();
      await act(async () => {
        save.resolve();
      });

      expect(answers).toEqual([false]);
      expect(onOpenChange).not.toHaveBeenCalled();
    });
  });

  it("takes its own close control and can go without one", () => {
    const { rerender } = render(
      <SlideOver open>
        <SlideOverContent>
          <SlideOverHeader
            close={
              <SlideOverCloseButton aria-label="Export schließen" disabled />
            }
          >
            <SlideOverTitle>Titel</SlideOverTitle>
          </SlideOverHeader>
        </SlideOverContent>
      </SlideOver>,
    );
    expect(
      screen.getByRole("button", { name: "Export schließen" }),
    ).toBeDisabled();
    rerender(
      <SlideOver open>
        <SlideOverContent>
          <SlideOverHeader close={false}>
            <SlideOverTitle>Titel</SlideOverTitle>
          </SlideOverHeader>
        </SlideOverContent>
      </SlideOver>,
    );
    expect(
      screen.queryByRole("button", { name: /schließen/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Titel")).toBeInTheDocument();
  });

  // #6567: a slide-over footer spaces its actions like the Modal and
  // FormModal footers (gap-3), not tighter.
  it("spaces footer actions with gap-3, like Modal and FormModal", () => {
    render(
      <SlideOverFooter data-testid="footer">
        <button type="button">Abbrechen</button>
        <button type="button">Speichern</button>
      </SlideOverFooter>,
    );
    const footer = screen.getByTestId("footer");
    expect(footer).toHaveClass("gap-3");
  });
});
