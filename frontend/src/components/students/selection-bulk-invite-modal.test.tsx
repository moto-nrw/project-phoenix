import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BulkInviteResult } from "~/lib/guardian-bulk-invite-api";
import { SelectionBulkInviteModal } from "./selection-bulk-invite-modal";

const { bulkInvite, toastSuccess } = vi.hoisted(() => ({
  bulkInvite: vi.fn(),
  toastSuccess: vi.fn(),
}));
vi.mock("~/lib/guardian-bulk-invite-api", () => ({
  bulkInviteGuardians: bulkInvite,
}));
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: toastSuccess, error: vi.fn() }),
}));

function result(overrides: Partial<BulkInviteResult>): BulkInviteResult {
  return {
    dryRun: true,
    invited: 0,
    linkedExistingAccount: 0,
    resent: 0,
    skippedActive: 0,
    skippedOpen: 0,
    skippedRestricted: 0,
    problems: [],
    ...overrides,
  };
}

function renderModal(onClose = vi.fn()) {
  render(
    <SelectionBulkInviteModal
      isOpen
      onClose={onClose}
      studentIds={["4", "9"]}
    />,
  );
  return onClose;
}

describe("SelectionBulkInviteModal", () => {
  beforeEach(() => vi.clearAllMocks());

  it("counts first and mails only after the click", async () => {
    bulkInvite
      .mockResolvedValueOnce(result({ invited: 3, skippedActive: 2 }))
      .mockResolvedValueOnce(
        result({ dryRun: false, invited: 2, linkedExistingAccount: 1 }),
      );
    renderModal();

    const send = await screen.findByRole("button", {
      name: "3 Eltern einladen",
    });
    expect(bulkInvite).toHaveBeenCalledTimes(1);
    expect(bulkInvite).toHaveBeenCalledWith(["4", "9"], {
      dryRun: true,
      resendOpen: false,
    });
    expect(screen.getByText("Nutzen das Elternportal schon")).toBeVisible();

    fireEvent.click(send);

    await waitFor(() =>
      expect(bulkInvite).toHaveBeenLastCalledWith(["4", "9"], {
        dryRun: false,
        resendOpen: false,
      }),
    );
    expect(
      await screen.findByText(
        "Die Einladung an 2 Eltern wird jetzt verschickt. Der Hinweis an 1 Elternteil mit moto-Konto wird jetzt verschickt. Das dauert ein paar Minuten.",
      ),
    ).toBeVisible();
    expect(toastSuccess).toHaveBeenCalledWith("3 Eltern eingeladen");
    expect(screen.getByRole("button", { name: "Schließen" })).toBeVisible();
  });

  it("offers to resend open invitations and recounts", async () => {
    let resolvePreview: (value: BulkInviteResult) => void;
    const reloadingPreview = new Promise<BulkInviteResult>((resolve) => {
      resolvePreview = resolve;
    });
    bulkInvite
      .mockResolvedValueOnce(result({ skippedOpen: 1 }))
      .mockReturnValueOnce(reloadingPreview);
    renderModal();

    const checkbox = await screen.findByLabelText(
      "Offene Einladungen noch einmal senden",
    );
    expect(screen.getByRole("button", { name: "Einladen" })).toBeDisabled();

    fireEvent.click(checkbox);

    expect(screen.getByRole("button", { name: "Einladen" })).toBeDisabled();

    resolvePreview!(result({ resent: 1 }));

    expect(
      await screen.findByRole("button", { name: "1 Elternteil einladen" }),
    ).toBeEnabled();
    expect(bulkInvite).toHaveBeenLastCalledWith(["4", "9"], {
      dryRun: true,
      resendOpen: true,
    });
  });

  it("describes existing-account notices without claiming a token invitation", async () => {
    bulkInvite
      .mockResolvedValueOnce(result({ invited: 1 }))
      .mockResolvedValueOnce(
        result({ dryRun: false, linkedExistingAccount: 1 }),
      );
    renderModal();

    fireEvent.click(
      await screen.findByRole("button", { name: "1 Elternteil einladen" }),
    );

    expect(
      await screen.findByText(
        "Der Hinweis an 1 Elternteil mit moto-Konto wird jetzt verschickt. Das dauert ein paar Minuten.",
      ),
    ).toBeVisible();
    expect(
      screen.queryByText(/Die Einladung an 0 Eltern/),
    ).not.toBeInTheDocument();
  });

  it("names the parents that cannot be invited and the child to fix", async () => {
    bulkInvite.mockResolvedValueOnce(
      result({
        invited: 1,
        problems: [
          {
            guardianProfileId: "7",
            guardianName: "Katharina Brenner",
            studentNames: ["Mia Brenner"],
            reason: "missing_email",
          },
        ],
      }),
    );
    renderModal();

    expect(await screen.findByText("Katharina Brenner")).toBeVisible();
    expect(
      screen.getByText(/\(Mia Brenner\): E-Mail-Adresse fehlt/),
    ).toBeVisible();
  });

  it("gives matching steps for each address problem", async () => {
    bulkInvite.mockResolvedValueOnce(
      result({
        problems: [
          {
            guardianProfileId: "7",
            guardianName: "Ohne Adresse",
            studentNames: ["Mia Brenner"],
            reason: "missing_email",
          },
          {
            guardianProfileId: "8",
            guardianName: "Falsche Adresse",
            studentNames: ["Noah Brenner"],
            reason: "invalid_email",
          },
          {
            guardianProfileId: "9",
            guardianName: "Doppelte Adresse",
            studentNames: ["Lina Brenner"],
            reason: "duplicate_email",
          },
        ],
      }),
    );
    renderModal();

    expect(
      await screen.findByText("Tragen Sie die E-Mail-Adresse beim Kind ein."),
    ).toBeVisible();
    expect(
      screen.getByText("Prüfen Sie die E-Mail-Adresse beim Kind."),
    ).toBeVisible();
    expect(
      screen.getByText("Prüfen Sie die E-Mail-Adressen bei den Eltern."),
    ).toBeVisible();
  });

  it("reports a failed run in the dialog and keeps it open", async () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    bulkInvite
      .mockResolvedValueOnce(result({ invited: 1 }))
      .mockRejectedValueOnce(new Error("boom"));
    const onClose = renderModal();

    fireEvent.click(
      await screen.findByRole("button", { name: "1 Elternteil einladen" }),
    );

    expect(
      await screen.findByText(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeVisible();
    expect(onClose).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });
});
