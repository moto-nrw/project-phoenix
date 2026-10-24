import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Input } from "~/components/ui/input";
import {
  loginUrl,
  ToastProvider,
  useApiErrorDisplay,
} from "~/contexts/ToastContext";
import { clientEnv } from "~/env.client";
import { ApiError } from "~/lib/api-error";
import { releaseFakeTimers } from "~/test/clock";

function TestForm({ error }: { error: ApiError }) {
  const display = useApiErrorDisplay();
  return (
    <form>
      <Input name="name" label="Name" error={display.fieldError("name")} />
      <Input name="age" label="Alter" error={display.fieldError("age")} />
      <button
        type="button"
        onClick={() => display.show(error, { object: "Die Gruppe" })}
      >
        Anzeigen
      </button>
    </form>
  );
}

describe("useApiErrorDisplay", () => {
  it("shows a message the screen found itself as a lasting error toast", async () => {
    vi.useFakeTimers();
    try {
      function TestAction() {
        const display = useApiErrorDisplay();
        return (
          <button
            type="button"
            onClick={() =>
              display.showMessage("Der Link konnte nicht kopiert werden.")
            }
          >
            Kopieren
          </button>
        );
      }
      render(
        <ToastProvider>
          <TestAction />
        </ToastProvider>,
      );
      fireEvent.click(screen.getByRole("button", { name: "Kopieren" }));
      act(() => {
        vi.advanceTimersByTime(10_000);
      });
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Der Link konnte nicht kopiert werden.",
      );
    } finally {
      releaseFakeTimers();
    }
  });

  it("shows field errors at fields and focuses the first failed field", async () => {
    const error = new ApiError("backend diagnostic", 422, {
      code: "general.input",
      errors: [
        { field: "age", reason: "English diagnostic" },
        { field: "name", reason: "Another diagnostic" },
      ],
    });
    render(
      <ToastProvider>
        <TestForm error={error} />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Anzeigen" }));
    const age = screen.getByRole("textbox", { name: "Alter" });
    await waitFor(() => expect(age).toHaveFocus());
    expect(age).toHaveAttribute("aria-invalid", "true");
    const describedBy = age.getAttribute("aria-describedby");
    expect(describedBy).toBe("age-error");
    expect(document.getElementById(describedBy!)).toHaveTextContent(
      "Bitte prüfen Sie dieses Feld.",
    );
    expect(
      screen.getByRole("alert", { name: /Die Gruppe/ }),
    ).toBeInTheDocument();
  });

  it("offers retry and a copyable request ID only for a server error", async () => {
    const clipboard = { writeText: vi.fn().mockResolvedValue(undefined) };
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: clipboard,
    });
    const retry = vi.fn();
    function TestAction() {
      const display = useApiErrorDisplay();
      return (
        <button
          onClick={() =>
            display.show(
              new ApiError("backend", 503, {
                code: "general.unavailable",
                instance: "req-20",
              }),
              { object: "Die Gruppe", retry },
            )
          }
        >
          Anzeigen
        </button>
      );
    }
    render(
      <ToastProvider>
        <TestAction />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Anzeigen" }));
    const copy = await screen.findByRole("button", {
      name: "Vorgangskennung kopieren",
    });
    expect(copy).toHaveTextContent("req-20");
    fireEvent.click(copy);
    expect(clipboard.writeText).toHaveBeenCalledWith("req-20");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("shows module booking refusals as informational feedback", async () => {
    function TestAction() {
      const display = useApiErrorDisplay();
      return (
        <button
          type="button"
          onClick={() =>
            void display.show(
              new ApiError("module not booked", 403, {
                code: "module.not_booked",
                details: { module: "zeiterfassung" },
              }),
              { object: "die Zeiterfassung" },
            )
          }
        >
          Anzeigen
        </button>
      );
    }

    render(
      <ToastProvider>
        <TestAction />
      </ToastProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Anzeigen" }));

    expect(
      await screen.findByRole("status", { name: /^Information:/ }),
    ).toHaveTextContent(
      "Für die Zeiterfassung fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("routes a 401 to the portal's login screen", () => {
    expect(loginUrl(clientEnv.NEXT_PUBLIC_PARENTS_HOSTNAME, "/children")).toBe(
      "/login?error=SessionExpired",
    );
    expect(loginUrl(clientEnv.NEXT_PUBLIC_SCHOOL_HOSTNAME, "/classes")).toBe(
      "/login?error=SessionExpired",
    );
    expect(loginUrl(clientEnv.NEXT_PUBLIC_OPERATOR_HOSTNAME, "/schools")).toBe(
      "/login?error=SessionExpired",
    );
    expect(loginUrl("localhost:3000", "/parents/children")).toBe(
      "/parents/login?error=SessionExpired",
    );
    expect(loginUrl("localhost:3000", "/home")).toBe("/?error=SessionExpired");
  });

  it("redirects after async classification of a 401 without showing a toast", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    let display: ReturnType<typeof useApiErrorDisplay> | undefined;
    function Probe() {
      display = useApiErrorDisplay();
      return <p>Fehleranzeige bereit</p>;
    }
    render(
      <ToastProvider>
        <Probe />
      </ToastProvider>,
    );
    // Die Probe ist montiert. Das belegt nur den Render; dass der
    // Toast-Bereich Fehler zeigen kann, belegt die Gegenprobe am Ende.
    expect(screen.getByText("Fehleranzeige bereit")).toBeInTheDocument();
    let presentation;
    await act(async () => {
      presentation = await display!.show(
        new ApiError("backend diagnostic", 401, { code: "general.permission" }),
        { object: "die Gruppe" },
      );
    });
    expect(presentation).toMatchObject({ kind: "api", requiresLogin: true });
    expect(assign).toHaveBeenCalledWith("/?error=SessionExpired");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();

    // Gegenprobe im selben Provider: ein anderer Fehler erscheint als Toast.
    await act(async () => {
      await display!.show(
        new ApiError("backend diagnostic", 500, { code: "general.server" }),
        { object: "die Gruppe" },
      );
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    assign.mockRestore();
  });
});
