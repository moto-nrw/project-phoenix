import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const swr = vi.hoisted(() => ({ useSWRAuth: vi.fn() }));

vi.mock("~/lib/swr", () => swr);

import { OwnQualificationsSection } from "./own-qualifications-section";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

function renderWithToast(ui: Parameters<typeof render>[0]) {
  return render(ui, { wrapper: ToastProvider });
}

describe("OwnQualificationsSection", () => {
  it("shows a failed load in place and retries it", async () => {
    const mutate = vi.fn();
    swr.useSWRAuth.mockReturnValue({
      data: undefined,
      error: new ApiError("boom", 500),
      mutate,
    });

    renderWithToast(<OwnQualificationsSection />);

    expect(
      await screen.findByText(catalogText("general.server", "meine Nachweise")),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalledOnce();
  });
});
