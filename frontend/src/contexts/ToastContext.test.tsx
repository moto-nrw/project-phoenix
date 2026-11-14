import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  act,
  fireEvent,
  renderHook,
  render,
  waitFor,
  screen,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { releaseFakeTimers } from "~/test/clock";
import { AT_LEAST_MD, BELOW_MD } from "~/lib/hooks/use-media-query";
import { ToastProvider, useToast } from "./ToastContext";

// Helper to create a wrapper with ToastProvider
function createWrapper() {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <ToastProvider>{children}</ToastProvider>;
  };
}

describe("ToastContext", () => {
  beforeEach(() => {
    // Mock matchMedia for toast visibility logic
    Object.defineProperty(globalThis, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: query === AT_LEAST_MD, // Desktop by default
        media: query,
        onchange: null,
        addListener: vi.fn(), // Deprecated
        removeListener: vi.fn(), // Deprecated
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    document.documentElement.lang = "de";
  });

  describe("useToast hook", () => {
    it("throws error when used outside ToastProvider", () => {
      // Suppress console.error for this test
      const consoleError = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      expect(() => {
        renderHook(() => useToast());
      }).toThrow("useToast must be used within ToastProvider");

      consoleError.mockRestore();
    });

    it("returns toast API when used inside ToastProvider", () => {
      const { result } = renderHook(() => useToast(), {
        wrapper: createWrapper(),
      });

      expect(result.current).toHaveProperty("success");
      expect(result.current).toHaveProperty("error");
      expect(result.current).toHaveProperty("info");
      expect(result.current).toHaveProperty("warning");
      expect(result.current).toHaveProperty("remove");
    });
  });

  describe("Toast functionality", () => {
    it("keeps error toasts until dismissed even if a caller requests a duration", () => {
      vi.useFakeTimers();
      function TestComponent() {
        const toast = useToast();
        return (
          <button onClick={() => toast.error("Fehler", { duration: 1 })}>
            Auslösen
          </button>
        );
      }
      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );
      fireEvent.click(screen.getByRole("button", { name: "Auslösen" }));
      expect(
        screen.getByRole("alert", { name: "Fehler: Fehler" }),
      ).toBeInTheDocument();
      act(() => vi.advanceTimersByTime(10_000));
      expect(
        screen.getByRole("alert", { name: "Fehler: Fehler" }),
      ).toBeInTheDocument();
    });

    it("dismisses success feedback after four seconds", () => {
      vi.useFakeTimers();
      function TestComponent() {
        const toast = useToast();
        return (
          <button onClick={() => toast.success("Gespeichert", { duration: 0 })}>
            Auslösen
          </button>
        );
      }
      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );
      fireEvent.click(screen.getByRole("button", { name: "Auslösen" }));
      expect(
        screen.getByRole("status", { name: "Erfolgreich!: Gespeichert" }),
      ).toBeInTheDocument();
      act(() => vi.advanceTimersByTime(3999));
      expect(
        screen.getByRole("status", { name: "Erfolgreich!: Gespeichert" }),
      ).toBeInTheDocument();
      act(() => vi.advanceTimersByTime(301));
      expect(
        screen.queryByRole("status", { name: "Erfolgreich!: Gespeichert" }),
      ).not.toBeInTheDocument();
    });
    it("ignores empty messages", async () => {
      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("");
        }, [toast]);

        return <div data-testid="test-component">Test</div>;
      }

      const { container } = render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      // Wait for component to render
      await waitFor(() => {
        expect(screen.getByTestId("test-component")).toBeInTheDocument();
      });

      // No toasts should be rendered when message is empty
      const toastContainer = container.querySelector('[class*="fixed"]');
      const toasts = toastContainer?.querySelectorAll("button, output");
      expect(toasts?.length ?? 0).toBe(0);
    });

    it("renders the desktop toast without an accessibility duplicate", async () => {
      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("Test message");
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      await waitFor(
        () => {
          expect(screen.getAllByText("Test message")).toHaveLength(1);
        },
        { timeout: 3000 },
      );
    });

    it("renders mobile notifications as non-blocking toast bars", async () => {
      vi.mocked(globalThis.matchMedia).mockImplementation((query: string) => ({
        matches: query === BELOW_MD,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }));

      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("Gespeichert", { duration: 0 });
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      const toast = await screen.findByRole("status", {
        name: "Erfolgreich!: Gespeichert",
      });
      expect(toast).toBeInTheDocument();
      expect(toast.parentElement?.parentElement).toHaveClass("mx-auto");
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      expect(document.documentElement.style.overflow).not.toBe("hidden");
    });

    it("uses the document language for mobile toast labels", async () => {
      vi.mocked(globalThis.matchMedia).mockImplementation((query: string) => ({
        matches: query === BELOW_MD,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }));
      document.documentElement.lang = "en";

      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("Saved", { duration: 0 });
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      expect(
        await screen.findByRole("status", { name: "Success!: Saved" }),
      ).toBeInTheDocument();
      expect(screen.getByLabelText("Close")).toBeInTheDocument();
    });

    it.each([
      [
        "de",
        "Vorgangskennung kopieren",
        "Kopiert.",
        "Kopiert die Kennung, damit Sie sie dem Support nennen können.",
      ],
      ["en", "Copy request ID", "Copied.", "Copy request ID"],
      [
        "tr",
        "İstek kimliğini kopyala",
        "Kopyalandı.",
        "İstek kimliğini kopyala",
      ],
    ])(
      "uses the document language %s for copy feedback and tooltips",
      async (locale, copyLabel, copiedLabel, tooltip) => {
        const originalClipboard = Object.getOwnPropertyDescriptor(
          navigator,
          "clipboard",
        );
        Object.defineProperty(navigator, "clipboard", {
          configurable: true,
          value: { writeText: vi.fn().mockResolvedValue(undefined) },
        });
        document.documentElement.lang = locale;
        function TestComponent() {
          const toast = useToast();
          return (
            <button
              onClick={() =>
                toast.error("Not available", {
                  requestId: "req-20",
                  requestIdLabel: "Request ID: {requestId}",
                })
              }
            >
              Show
            </button>
          );
        }
        try {
          render(
            <ToastProvider>
              <TestComponent />
            </ToastProvider>,
          );
          fireEvent.click(screen.getByRole("button", { name: "Show" }));
          const copyButton = screen.getByRole("button", { name: copyLabel });
          fireEvent.keyDown(document, { key: "Tab" });
          fireEvent.focus(copyButton);
          expect(
            document.querySelector("[data-action-hint]")?.textContent,
          ).toBe(tooltip);
          fireEvent.click(copyButton);
          expect(await screen.findByRole("status")).toHaveTextContent(
            copiedLabel,
          );
        } finally {
          if (originalClipboard) {
            Object.defineProperty(navigator, "clipboard", originalClipboard);
          } else {
            Reflect.deleteProperty(navigator, "clipboard");
          }
        }
      },
    );

    it.each([
      {
        locale: "pl",
        dialogTitle: "Powiadomienia",
        close: "Zamknij",
        dismissMessage: "Zapisano",
        actionMessage: "Gotowe",
        action: "Otwórz",
        dismissLabel: "Sukces!: Zapisano",
        actionLabel: "Dotknij: Otwórz",
      },
      {
        locale: "tr",
        dialogTitle: "Bildirimler",
        close: "Kapat",
        dismissMessage: "Kaydedildi",
        actionMessage: "Hazır",
        action: "Aç",
        dismissLabel: "Başarılı!: Kaydedildi",
        actionLabel: "Dokunun: Aç",
      },
      {
        locale: "uk",
        dialogTitle: "Сповіщення",
        close: "Закрити",
        dismissMessage: "Збережено",
        actionMessage: "Готово",
        action: "Відкрити",
        dismissLabel: "Успішно!: Збережено",
        actionLabel: "Торкніться: Відкрити",
      },
    ])(
      "localizes all mobile toast chrome for $locale",
      async ({
        locale,
        close,
        dismissMessage,
        actionMessage,
        action,
        dismissLabel,
        actionLabel,
      }) => {
        vi.mocked(globalThis.matchMedia).mockImplementation(
          (query: string) => ({
            matches: query === BELOW_MD,
            media: query,
            onchange: null,
            addListener: vi.fn(),
            removeListener: vi.fn(),
            addEventListener: vi.fn(),
            removeEventListener: vi.fn(),
            dispatchEvent: vi.fn(),
          }),
        );
        document.documentElement.lang = locale;

        function TestComponent() {
          const toast = useToast();

          useEffect(() => {
            toast.success(dismissMessage, { duration: 0 });
            toast.success(actionMessage, {
              duration: 0,
              action: { label: action, onClick: vi.fn() },
            });
          }, [toast]);

          return null;
        }

        render(
          <ToastProvider>
            <TestComponent />
          </ToastProvider>,
        );

        expect(screen.getAllByLabelText(close)).toHaveLength(2);
        expect(screen.getByLabelText(dismissLabel)).toBeInTheDocument();
        expect(screen.getByLabelText(actionLabel)).toBeInTheDocument();
      },
    );

    it("keeps remaining mobile toast bars visible after dismissing one", async () => {
      vi.mocked(globalThis.matchMedia).mockImplementation((query: string) => ({
        matches: query === BELOW_MD,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }));

      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("First", { duration: 0 });
          toast.success("Second", { duration: 0 });
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      await screen.findByText("Second");
      // Fake timers skip the 300-ms exit animation of the dismissed bar.
      vi.useFakeTimers();
      fireEvent.click(screen.getAllByLabelText("Schließen")[1]!);
      await act(() => vi.advanceTimersByTimeAsync(300));
      releaseFakeTimers();

      await waitFor(() => {
        expect(screen.queryByText("Second")).not.toBeInTheDocument();
        expect(screen.getByText("First")).toBeVisible();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      });
    });

    it("runs a mobile toast action without blocking the page", async () => {
      vi.mocked(globalThis.matchMedia).mockImplementation((query: string) => ({
        matches: query === BELOW_MD,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }));
      const onAction = vi.fn();
      const onContinue = vi.fn();

      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.info("Änderung gespeichert", {
            duration: 0,
            action: { label: "Rückgängig", onClick: onAction },
          });
        }, [toast]);

        return (
          <button type="button" onClick={onContinue}>
            Weiterarbeiten
          </button>
        );
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      await screen.findByText("Änderung gespeichert");
      const continueButton = screen.getByRole("button", {
        name: "Weiterarbeiten",
      });
      fireEvent.click(continueButton);
      expect(onContinue).toHaveBeenCalledOnce();

      const toast = screen.getByRole("status", {
        name: "Information: Änderung gespeichert",
      });
      expect(toast.parentElement?.parentElement).toHaveClass(
        "pointer-events-none",
      );
      // #5531: 44 px unter Fingerbedienung trägt die Fingerregel des Kits.
      expect(screen.getByLabelText("Schließen")).toHaveClass(
        "pointer-coarse:min-h-11",
        "pointer-coarse:min-w-11",
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Tippen zum Rückgängig" }),
      );
      expect(onAction).toHaveBeenCalledOnce();
    });

    it("runs a toast action from the desktop toast", async () => {
      const onAction = vi.fn();

      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.info("Test message", {
            duration: 0,
            action: { label: "Öffnen", onClick: onAction },
          });
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      const action = await screen.findByRole("button", { name: "Öffnen" });
      expect(action).toHaveClass("self-center");
      fireEvent.click(action);

      expect(onAction).toHaveBeenCalledOnce();
    });
  });

  // #6454: Rückmeldungen gehen bei offenen Fehlern und schnellen
  // Wiederholungen nicht verloren.
  describe("Fehlerstau und Wiederholen", () => {
    let api: ReturnType<typeof useToast> | undefined;
    function Probe() {
      api = useToast();
      return <p>Toast-Bereich bereit</p>;
    }
    function renderProbe() {
      render(
        <ToastProvider>
          <Probe />
        </ToastProvider>,
      );
      expect(screen.getByText("Toast-Bereich bereit")).toBeInTheDocument();
      return api!;
    }

    it("shows a success toast next to three open errors without evicting one", () => {
      const toast = renderProbe();
      act(() => {
        toast.error("Fehler A");
        toast.error("Fehler B");
        toast.error("Fehler C");
        toast.success("Gespeichert");
      });

      const success = screen.getByRole("status", {
        name: "Erfolgreich!: Gespeichert",
      });
      expect(success).toHaveAttribute("aria-live", "polite");
      expect(screen.getAllByRole("alert")).toHaveLength(3);
      for (const message of ["Fehler A", "Fehler B", "Fehler C"]) {
        expect(
          screen.getByRole("alert", { name: `Fehler: ${message}` }),
        ).toHaveAttribute("aria-live", "assertive");
      }
    });

    it("keeps only the newest non-error toast in the extra slot", () => {
      const toast = renderProbe();
      act(() => {
        toast.error("Fehler A");
        toast.error("Fehler B");
        toast.error("Fehler C");
        toast.success("Gespeichert");
        toast.info("Liste aktualisiert", { duration: 0 });
      });

      expect(
        screen.getByRole("status", {
          name: "Information: Liste aktualisiert",
        }),
      ).toBeInTheDocument();
      expect(screen.getAllByRole("status")).toHaveLength(1);
      expect(screen.getAllByRole("alert")).toHaveLength(3);
    });

    it("keeps the newest non-error toast visible when another error arrives", () => {
      const toast = renderProbe();
      act(() => {
        toast.error("Fehler A");
        toast.success("Gespeichert");
        toast.error("Fehler B");
        toast.error("Fehler C");
      });

      expect(
        screen.getByRole("status", { name: "Erfolgreich!: Gespeichert" }),
      ).toBeInTheDocument();
      expect(screen.getAllByRole("alert")).toHaveLength(3);
    });

    it("still drops the oldest non-error toast when errors leave room", () => {
      const toast = renderProbe();
      act(() => {
        toast.error("Fehler A");
        toast.info("Eins", { duration: 0 });
        toast.info("Zwei", { duration: 0 });
        toast.info("Drei", { duration: 0 });
      });

      expect(
        screen.getByRole("alert", { name: "Fehler: Fehler A" }),
      ).toBeInTheDocument();
      expect(
        screen
          .getAllByRole("status")
          .map((item) => item.getAttribute("aria-label")),
      ).toEqual(["Information: Zwei", "Information: Drei"]);
    });

    it("still drops the same message fired twice within two seconds", () => {
      const toast = renderProbe();
      act(() => {
        toast.error("Die Gruppe konnte nicht gespeichert werden.");
        toast.error("Die Gruppe konnte nicht gespeichert werden.");
      });

      expect(screen.getAllByRole("alert")).toHaveLength(1);
    });

    it("shows a quickly failing retry as a new error toast", () => {
      vi.useFakeTimers();
      const message = "Die Gruppe konnte nicht gespeichert werden.";
      const toast = renderProbe();
      const retry = vi.fn(() => toast.error(message, { action }));
      const action = { label: "Wiederholen", onClick: retry };
      act(() => toast.error(message, { action }));
      const first = screen.getByRole("alert", { name: `Fehler: ${message}` });

      act(() => vi.advanceTimersByTime(500));
      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

      expect(retry).toHaveBeenCalledOnce();
      // Genau ein aktueller Fehler für die Aktion, als neues Element, damit
      // ein Screenreader die Meldung erneut ansagt.
      const alerts = screen.getAllByRole("alert", {
        name: `Fehler: ${message}`,
      });
      expect(alerts).toHaveLength(1);
      expect(alerts[0]).not.toBe(first);
      expect(alerts[0]).toHaveAttribute("aria-live", "assertive");

      // Die neue Meldung bleibt, auch nach dem Ausblenden der alten.
      act(() => vi.advanceTimersByTime(10_000));
      expect(screen.getByRole("alert", { name: `Fehler: ${message}` })).toBe(
        alerts[0],
      );
    });

    it("still drops a duplicate the retry itself fires twice", () => {
      const message = "Die Gruppe konnte nicht gespeichert werden.";
      const toast = renderProbe();
      const retry = vi.fn(() => {
        toast.error(message);
        toast.error(message);
      });
      act(() =>
        toast.error(message, {
          action: { label: "Wiederholen", onClick: retry },
        }),
      );

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

      expect(retry).toHaveBeenCalledOnce();
      expect(
        screen.getAllByRole("alert", { name: `Fehler: ${message}` }),
      ).toHaveLength(1);
    });
  });

  describe("Accessibility", () => {
    it("has proper ARIA attributes", async () => {
      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("Test message");
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      await waitFor(
        () => {
          const outputs = screen.getAllByRole("status");
          expect(outputs.length).toBeGreaterThan(0);

          outputs.forEach((output) => {
            expect(output).toHaveAttribute("aria-live", "polite");
            expect(output).toHaveAttribute("aria-atomic", "true");
          });
        },
        { timeout: 3000 },
      );
    });

    it.each([
      ["de", "Schließen"],
      ["en", "Close"],
      ["ru", "Закрыть"],
      ["sq", "Mbyll"],
      ["pl", "Zamknij"],
      ["tr", "Kapat"],
      ["uk", "Закрити"],
    ])(
      "has a localized desktop close button for %s",
      async (locale, closeLabel) => {
        document.documentElement.lang = locale;

        function TestComponent() {
          const toast = useToast();

          useEffect(() => {
            toast.success("Test message");
          }, [toast]);

          return null;
        }

        render(
          <ToastProvider>
            <TestComponent />
          </ToastProvider>,
        );

        await waitFor(
          () => {
            const closeButtons = screen.getAllByLabelText(closeLabel);
            expect(closeButtons.length).toBeGreaterThan(0);
          },
          { timeout: 3000 },
        );
      },
    );
  });
});
