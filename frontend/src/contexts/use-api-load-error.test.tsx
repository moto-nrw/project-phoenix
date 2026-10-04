import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { ToastProvider, useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";

function TestSection({
  error,
  retry,
}: {
  readonly error: unknown;
  readonly retry?: () => void;
}) {
  const load = useApiLoadError();
  return (
    <section>
      <LoadErrorAlert error={load.error} />
      <Input name="first_name" label="Vorname" />
      <button
        type="button"
        onClick={() => void load.show(error, { object: "die Liste", retry })}
      >
        Laden
      </button>
      <button type="button" onClick={load.clear}>
        Leeren
      </button>
    </section>
  );
}

function renderSection(error: unknown, retry?: () => void) {
  return render(
    <ToastProvider>
      <TestSection error={error} retry={retry} />
    </ToastProvider>,
  );
}

describe("useApiLoadError", () => {
  it("shows a failed load in place with retry and request ID, never as a toast", async () => {
    const retry = vi.fn();
    renderSection(
      new ApiError("boom", 503, {
        code: "general.unavailable",
        instance: "req-503",
      }),
      retry,
    );

    fireEvent.click(screen.getByRole("button", { name: "Laden" }));

    expect(
      await screen.findByText(
        "Die Liste ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("Vorgangskennung: req-503");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(
      screen.queryByRole("alert", { name: /^Fehler:/ }),
    ).not.toBeInTheDocument();
  });

  it("does not mark or focus fields of the surrounding page", async () => {
    renderSection(
      new ApiError("bad", 400, {
        code: "general.input",
        errors: [{ field: "first_name", reason: "invalid" }],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Laden" }));

    await screen.findByText(/konnte nicht übernommen werden/);
    const first = screen.getByRole("textbox", { name: "Vorname" });
    expect(first).not.toHaveFocus();
    expect(first).not.toHaveAttribute("aria-invalid");
  });

  it("clears the message", async () => {
    renderSection(new ApiError("boom", 500, { code: "general.server" }));
    fireEvent.click(screen.getByRole("button", { name: "Laden" }));
    await screen.findByText(/konnte nicht bearbeitet werden/);

    fireEvent.click(screen.getByRole("button", { name: "Leeren" }));

    await waitFor(() =>
      expect(
        screen.queryByText(/konnte nicht bearbeitet werden/),
      ).not.toBeInTheDocument(),
    );
  });
});
