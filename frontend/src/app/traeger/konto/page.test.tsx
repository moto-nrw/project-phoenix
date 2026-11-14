import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";

const api = vi.hoisted(() => ({
  fetchTraegerProfile: vi.fn(),
  changeTraegerProfileName: vi.fn(),
}));

vi.mock("~/lib/traeger-api", () => api);
// The device list has its own tests; here only its place on the page counts.
vi.mock("~/components/settings/trusted-devices-section", () => ({
  TrustedDevicesSection: ({ scope }: { scope: string }) => (
    <div data-testid="trusted-devices" data-scope={scope} />
  ),
}));

import TraegerAccountPage from "./page";

const PROFILE = {
  accountId: "4",
  email: "kasse@traeger.example",
  name: "",
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerAccountPage />
    </ToastProvider>,
  );
}

describe("TraegerAccountPage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    api.fetchTraegerProfile.mockResolvedValue(PROFILE);
  });

  it("shows the sign-in address and saves the own name", async () => {
    api.changeTraegerProfileName.mockResolvedValue({
      ...PROFILE,
      name: "Karl Kasse",
    });
    renderPage();

    expect(
      await screen.findByText("kasse@traeger.example"),
    ).toBeInTheDocument();
    // The person sees and removes the devices remembered here (#6955).
    expect(screen.getByTestId("trusted-devices")).toHaveAttribute(
      "data-scope",
      "traeger",
    );
    const field = screen.getByRole("textbox", { name: /Ihr Name/ });
    // Anker: das Feld ist da; gespeichert wird erst nach einer Änderung.
    expect(field).toHaveValue("");
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();

    fireEvent.change(field, { target: { value: " Karl Kasse " } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.changeTraegerProfileName).toHaveBeenCalledWith("Karl Kasse"),
    );
    expect(
      await screen.findByText("Ihr Name ist gespeichert."),
    ).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /Ihr Name/ })).toHaveValue(
      "Karl Kasse",
    );
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
  });

  it("asks for a name before it saves", async () => {
    api.fetchTraegerProfile.mockResolvedValue({ ...PROFILE, name: "Karl" });
    renderPage();

    fireEvent.change(await screen.findByRole("textbox", { name: /Ihr Name/ }), {
      target: { value: "   " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      (await screen.findAllByText("Bitte geben Sie Ihren Namen ein."))[0],
    ).toBeInTheDocument();
    expect(api.changeTraegerProfileName).not.toHaveBeenCalled();
  });

  it("shows the backend's refusal at the field", async () => {
    api.changeTraegerProfileName.mockRejectedValue(
      new ApiError("Bad Request", 400, {
        code: "general.input",
        errors: [{ field: "name", reason: "invalid" }],
      }),
    );
    renderPage();

    fireEvent.change(await screen.findByRole("textbox", { name: /Ihr Name/ }), {
      target: { value: "Karl\u0007" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(screen.getByRole("textbox", { name: /Ihr Name/ })).toHaveAttribute(
        "aria-invalid",
        "true",
      ),
    );
    // The draft stays for a correction.
    expect(screen.getByRole("textbox", { name: /Ihr Name/ })).toHaveValue(
      "Karl\u0007",
    );
  });

  it("shows a load error instead of the cards", async () => {
    api.fetchTraegerProfile.mockRejectedValue(
      new ApiError("Unavailable", 503, { code: "general.unavailable" }),
    );
    renderPage();

    expect((await screen.findAllByRole("alert"))[0]).toBeInTheDocument();
    expect(
      screen.queryByRole("textbox", { name: /Ihr Name/ }),
    ).not.toBeInTheDocument();
  });
});
