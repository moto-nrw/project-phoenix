import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { ToastProvider, useApiFormError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";

function TestForm({
  error,
  retry,
}: {
  readonly error: unknown;
  readonly retry?: () => void;
}) {
  const errors = useApiFormError();
  return (
    <form>
      <FormErrorAlert message={errors.error} />
      <Input
        name="first_name"
        label="Vorname"
        error={errors.fieldError("first_name")}
      />
      <Input
        name="last_name"
        label="Nachname"
        error={errors.fieldError("last_name")}
      />
      <Input name="email" label="E-Mail" error={errors.fieldError("email")} />
      <button
        type="button"
        onClick={() => void errors.show(error, { object: "die Person", retry })}
      >
        Speichern
      </button>
      <button type="button" onClick={errors.clear}>
        Abbrechen
      </button>
    </form>
  );
}

function renderForm(error: unknown, retry?: () => void) {
  return render(
    <ToastProvider>
      <TestForm error={error} retry={retry} />
    </ToastProvider>,
  );
}

const toastAlert = () => screen.queryByRole("alert", { name: /^Fehler:/ });

describe("useApiFormError", () => {
  it("marks every blank field, focuses the first and reports in the form, not in a toast", async () => {
    renderForm(
      new ApiError("validation failed", 400, {
        code: "general.input",
        errors: [
          { field: "email", reason: "is required" },
          { field: "first_name", reason: "is required" },
          { field: "last_name", reason: "is required" },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const email = screen.getByRole("textbox", { name: "E-Mail" });
    await waitFor(() => expect(email).toHaveFocus());
    for (const name of ["Vorname", "Nachname", "E-Mail"]) {
      const field = screen.getByRole("textbox", { name });
      expect(field).toHaveAttribute("aria-invalid", "true");
      expect(
        document.getElementById(field.getAttribute("aria-describedby")!),
      ).toHaveTextContent("Bitte prüfen Sie dieses Feld.");
    }
    expect(
      screen.getByText(
        "Die Person konnte nicht übernommen werden. Bitte prüfen Sie Ihre Angaben.",
      ),
    ).toBeInTheDocument();
    expect(toastAlert()).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Vorgangskennung kopieren" }),
    ).not.toBeInTheDocument();
  });

  it("names the permission class and its next step without a request ID", async () => {
    renderForm(
      new ApiError("forbidden", 403, {
        code: "general.permission",
        instance: "req-403",
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Für die Person fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/req-403/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
  });

  it("shows a copyable request ID and a retry for a server error", async () => {
    const clipboard = { writeText: vi.fn().mockResolvedValue(undefined) };
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: clipboard,
    });
    const retry = vi.fn();
    renderForm(
      new ApiError("boom", 500, {
        code: "general.server",
        instance: "req-500",
      }),
      retry,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const copy = await screen.findByRole("button", {
      name: "Vorgangskennung kopieren",
    });
    expect(copy).toHaveTextContent("Vorgangskennung: req-500");
    fireEvent.click(copy);
    expect(clipboard.writeText).toHaveBeenCalledWith("req-500");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(toastAlert()).not.toBeInTheDocument();
  });

  it("clears the form error and the field errors", async () => {
    renderForm(
      new ApiError("validation failed", 400, {
        code: "general.input",
        errors: [{ field: "first_name", reason: "is required" }],
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    const first = screen.getByRole("textbox", { name: "Vorname" });
    await waitFor(() => expect(first).toHaveAttribute("aria-invalid", "true"));

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(first).not.toHaveAttribute("aria-invalid");
    expect(
      screen.queryByText(/konnte nicht übernommen werden/),
    ).not.toBeInTheDocument();
  });

  it("reports a crash without a code in the form", async () => {
    renderForm(new TypeError("undefined is not a function"));

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Person konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    expect(toastAlert()).not.toBeInTheDocument();
  });
});
