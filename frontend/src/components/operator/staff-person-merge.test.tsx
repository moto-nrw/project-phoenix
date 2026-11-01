vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "account-1", tenantId: 1, scope: "operator" },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { StaffMergeCandidate } from "~/lib/operator/staff-person-merge-api";

vi.unmock("swr");

const { preview, execute } = vi.hoisted(() => ({
  preview: vi.fn(),
  execute: vi.fn(),
}));
vi.mock("~/lib/operator/staff-person-merge-api", () => ({
  previewStaffPersonMerges: preview,
  executeStaffPersonMerges: execute,
}));
import { StaffPersonMerge } from "./staff-person-merge";

function candidate(name: string, id: string): StaffMergeCandidate {
  return {
    revision: `review-${id}`,
    keeper: {
      personId: id,
      name,
      accountId: `account-${id}`,
      createdAt: "2026-04-14T12:00:00Z",
      staffId: `current-${id}`,
      staffCreatedAt: "2026-04-14T12:00:00Z",
      retiredAt: null,
    },
    donor: {
      personId: `old-${id}`,
      name,
      createdAt: "2026-09-01T12:00:00Z",
      staffId: `former-${id}`,
      staffCreatedAt: "2026-09-01T12:00:00Z",
      retiredAt: "2026-09-02T12:00:00Z",
    },
  };
}
const anna = candidate("Anna Beispiel", "11");
const ben = candidate("Ben Beispiel", "12");
function setup() {
  const onMerged = vi.fn().mockResolvedValue(undefined);
  render(
    <SWRConfig value={{ provider: () => new Map(), shouldRetryOnError: false }}>
      <ToastProvider>
        <StaffPersonMerge
          schoolId="6"
          schoolName="Schule Sechs"
          onMerged={onMerged}
        />
      </ToastProvider>
    </SWRConfig>,
  );
  return { onMerged };
}
function submit() {
  return screen.getByRole("button", { name: /geprüfte Fälle zusammenführen/ });
}
async function reviewAnna() {
  fireEvent.click(
    await screen.findByRole("checkbox", {
      name: /Beide Einträge von Anna Beispiel/,
    }),
  );
  fireEvent.click(
    screen.getByRole("checkbox", { name: /mit Schule Sechs abgestimmt/ }),
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  preview.mockResolvedValue([anna, ben]);
  execute.mockResolvedValue(undefined);
});

describe("StaffPersonMerge", () => {
  it("requires individual identity review and consultation, and submits only selected cases", async () => {
    const { onMerged } = setup();
    const review = await screen.findByRole("checkbox", {
      name: /Beide Einträge von Anna Beispiel/,
    });
    expect(submit()).toBeDisabled();
    fireEvent.click(review);
    expect(submit()).toBeDisabled();
    fireEvent.click(
      screen.getByRole("checkbox", { name: /mit Schule Sechs abgestimmt/ }),
    );
    expect(submit()).toBeEnabled();
    preview.mockResolvedValue([]);
    fireEvent.click(submit());
    await waitFor(() =>
      expect(execute).toHaveBeenCalledExactlyOnceWith("6", [anna], true),
    );
    expect(
      await screen.findByText(
        "Die ausgewählten Personen wurden zusammengeführt.",
      ),
    ).toBeInTheDocument();
    expect(onMerged).toHaveBeenCalledOnce();
    expect(
      await screen.findByText("Keine passenden Doppelsätze gefunden."),
    ).toBeInTheDocument();
  });
  it("clears identity review and school confirmation after a stale preview", async () => {
    const { onMerged } = setup();
    await reviewAnna();
    execute.mockRejectedValueOnce(new ApiError("preview changed", 409));
    fireEvent.click(submit());
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.getByRole("checkbox", {
          name: /Beide Einträge von Anna Beispiel/,
        }),
      ).not.toBeChecked(),
    );
    expect(
      screen.getByRole("checkbox", { name: /mit Schule Sechs abgestimmt/ }),
    ).not.toBeChecked();
    expect(submit()).toBeDisabled();
    expect(onMerged).not.toHaveBeenCalled();
    expect(
      screen.queryByText("Die ausgewählten Personen wurden zusammengeführt."),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Vorschläge neu laden" }),
    );
    await waitFor(() => expect(preview).toHaveBeenCalledTimes(2));
  });
  it("prevents a second submission while the confirmed batch is being saved", async () => {
    setup();
    let finish: (() => void) | undefined;
    execute.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    );
    await reviewAnna();
    const button = submit();
    fireEvent.click(button);
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(execute).toHaveBeenCalledOnce();
    finish?.();
    expect(
      await screen.findByText(
        "Die ausgewählten Personen wurden zusammengeführt.",
      ),
    ).toBeInTheDocument();
  });
  it("keeps a completed merge successful when refreshing the lists fails", async () => {
    const { onMerged } = setup();
    onMerged.mockRejectedValueOnce(new Error("refresh failed"));
    await reviewAnna();
    fireEvent.click(submit());
    expect(
      await screen.findByText(
        "Die ausgewählten Personen wurden zusammengeführt.",
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("alert", {
        name: /Listen konnten nicht vollständig aktualisiert werden/,
      }),
    ).toBeInTheDocument();
    expect(execute).toHaveBeenCalledOnce();
  });
  it("requires a new review after the operator reloads suggestions", async () => {
    setup();
    await reviewAnna();
    expect(submit()).toBeEnabled();
    fireEvent.click(
      screen.getByRole("button", { name: "Vorschläge neu laden" }),
    );
    await waitFor(() => expect(preview).toHaveBeenCalledTimes(2));
    expect(submit()).toBeDisabled();
    expect(
      screen.getByRole("checkbox", { name: /mit Schule Sechs abgestimmt/ }),
    ).not.toBeChecked();
  });
});
