import type { ReactNode } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { StudentCreateModal } from "./student-create-modal";

vi.mock("~/lib/student-arrival-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-arrival-api")>()),
  fetchArrivalSettings: vi.fn(async () => ({
    care_days_source: "weekly_plan",
  })),
}));

// Keep the form and its controls real; only replace Vaul's animation shell.
vi.mock("~/components/ui/slide-over", () => {
  const Container = ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  );
  return {
    SlideOver: ({ open, children }: { open: boolean; children: ReactNode }) =>
      open ? <div role="dialog">{children}</div> : null,
    SlideOverContent: Container,
    SlideOverHeader: Container,
    SlideOverFooter: Container,
    SlideOverTitle: Container,
    SlideOverCloseButton: () => <button type="button">Schließen</button>,
  };
});

describe("student creation required fields", () => {
  it("marks and focuses the missing last name before sending", async () => {
    const onCreate = vi.fn();
    render(<StudentCreateModal isOpen onClose={vi.fn()} onCreate={onCreate} />);

    const firstName = await screen.findByRole("textbox", { name: /Vorname/ });
    fireEvent.change(firstName, { target: { value: "Mia" } });
    fireEvent.change(screen.getByRole("textbox", { name: /Klasse/ }), {
      target: { value: "2b" },
    });
    fireEvent.submit(firstName.closest("form")!);

    const lastName = screen.getByRole("textbox", { name: /Nachname/ });
    expect(lastName).toHaveAttribute("aria-invalid", "true");
    expect(lastName).toHaveAccessibleDescription(
      /Bitte geben Sie den Nachnamen ein\./,
    );
    await waitFor(() => expect(lastName).toHaveFocus());
    // Anker: the real last-name control displays its local validation hint.
    expect(
      screen.getByText("Bitte geben Sie den Nachnamen ein."),
    ).toBeVisible();
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("shows a BFF last_name error at the real last-name control and keeps the draft", async () => {
    const onCreate = vi.fn().mockRejectedValueOnce(
      new ApiError("Last name is required", 400, {
        code: "general.input",
        errors: [{ field: "last_name", reason: "required" }],
      }),
    );
    render(<StudentCreateModal isOpen onClose={vi.fn()} onCreate={onCreate} />);

    const firstName = await screen.findByRole("textbox", { name: /Vorname/ });
    const lastName = screen.getByRole("textbox", { name: /Nachname/ });
    fireEvent.change(firstName, { target: { value: "Mia" } });
    fireEvent.change(lastName, { target: { value: "Muster" } });
    fireEvent.change(screen.getByRole("textbox", { name: /Klasse/ }), {
      target: { value: "2b" },
    });
    fireEvent.submit(firstName.closest("form")!);

    await waitFor(() =>
      expect(lastName).toHaveAttribute("aria-invalid", "true"),
    );
    expect(onCreate).toHaveBeenCalledOnce();
    expect(lastName).toHaveAccessibleDescription(
      "Bitte prüfen Sie dieses Feld.",
    );
    await waitFor(() => expect(lastName).toHaveFocus());
    expect(firstName).toHaveValue("Mia");
    expect(lastName).toHaveValue("Muster");
  });
});
