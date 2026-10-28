import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { LogoutModal } from "./logout-modal";

const { mockLogout, presenterReady, releasePresenter } = vi.hoisted(() => {
  let releasePresenter!: () => void;
  const presenterReady = new Promise<void>((resolve) => {
    releasePresenter = resolve;
  });
  return { mockLogout: vi.fn(), presenterReady, releasePresenter };
});

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ logout: mockLogout }),
}));

vi.mock("~/lib/error-presentation", async (importOriginal) => {
  await presenterReady;
  return importOriginal();
});

it("shows a delayed logout error and retry after the dialog closes", async () => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
  mockLogout
    .mockRejectedValueOnce(
      new ApiError("revocation unavailable", 503, {
        code: "general.unavailable",
      }),
    )
    .mockResolvedValueOnce(undefined);

  function Screen() {
    const [isOpen, setIsOpen] = useState(true);
    return (
      <ToastProvider>
        <p>Angemeldet</p>
        <LogoutModal isOpen={isOpen} onClose={() => setIsOpen(false)} />
      </ToastProvider>
    );
  }

  render(<Screen />);
  expect(screen.getByRole("button", { name: "Abmelden" })).toBeInTheDocument();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Abmelden" }));
  });
  // Anker: Die angemeldete Seite bleibt nach dem fehlgeschlagenen Logout da.
  expect(screen.getByText("Angemeldet")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Abmelden" })).toBeNull();

  await act(async () => {
    releasePresenter();
    await vi.dynamicImportSettled();
  });
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Die Abmeldung ist gerade nicht erreichbar",
  );
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
  });
  expect(mockLogout).toHaveBeenCalledTimes(2);
});
