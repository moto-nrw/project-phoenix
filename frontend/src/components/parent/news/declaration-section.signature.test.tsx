import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { NewsDetailModal } from "./news-components";
import { declarationErrorOutcome } from "./declaration-section";
import { ApiError } from "~/lib/api-error";
import type { ParentAnnouncement } from "~/lib/parent-api";
import * as parentApi from "~/lib/parent-api";
import { catalogText } from "~/test/error-catalog-text";

// Einverständnis mit Unterschrift (#4785): agreeing needs a confirmed
// signature in the confirmation; declining goes without one. A refused
// signature keeps the dialog open and asks for a new one.

const PNG_DATA_URL = "data:image/png;base64,iVBORw0KGgo=";

function item(): ParentAnnouncement {
  return {
    id: "42",
    title: "Ausflug in den Zoo",
    body: "Wir fahren am Freitag in den Zoo.",
    priority: "info",
    requires_acknowledgement: false,
    delivery_mode: "declaration",
    school_name: "OGS Am Berg",
    published_at: "2026-09-01T08:00:00Z",
    read: true,
    acknowledged: false,
    response_type: "none",
    declaration: {
      kind: "consent",
      signers: "any",
      revocable: true,
      requires_password: false,
      requires_signature: true,
      deadline: null,
      closed: false,
      version: { id: "12", version_no: 2, content_hash: "ab".repeat(32) },
      children: [
        {
          student_id: "5",
          first_name: "Mia",
          last_name: "Muster",
          can_submit: true,
          state: "open",
          my_action: null,
          my_submitted_at: null,
          allowed_actions: ["agreed", "declined"],
          other_signers: [],
        },
      ],
    },
  };
}

function submission(action: "agreed" | "declined") {
  return {
    submission: {
      id: "77",
      action,
      submitted_at: "2026-09-09T08:30:00Z",
      version_no: 2,
      content_hash: "ab".repeat(32),
      record_hash: "cd".repeat(32),
      password_confirmed: false,
    },
    created: true,
  };
}

beforeEach(() => {
  vi.spyOn(parentApi, "listAnnouncementAttachments").mockResolvedValue([]);
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(
    () =>
      ({
        clearRect: vi.fn(),
        beginPath: vi.fn(),
        moveTo: vi.fn(),
        lineTo: vi.fn(),
        stroke: vi.fn(),
        fillText: vi.fn(),
        measureText: vi.fn(() => ({ width: 10 })),
      }) as unknown as CanvasRenderingContext2D,
  );
  vi.spyOn(HTMLCanvasElement.prototype, "toDataURL").mockReturnValue(
    PNG_DATA_URL,
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

function sign(dialog: HTMLElement) {
  const canvas = within(dialog).getByRole("img", {
    name: "Feld zum Unterschreiben",
  });
  fireEvent.pointerDown(canvas, { clientX: 5, clientY: 5, pointerId: 1 });
  fireEvent.pointerMove(canvas, { clientX: 30, clientY: 20, pointerId: 1 });
  fireEvent.pointerUp(canvas, { pointerId: 1 });
  fireEvent.click(
    within(dialog).getByRole("button", { name: "Unterschrift bestätigen" }),
  );
}

describe("Einverständnis mit Unterschrift (#4785)", () => {
  it("says before answering that agreeing is signed", () => {
    render(
      <NewsDetailModal item={item()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );
    expect(
      screen.getByText(
        "Zum Zustimmen unterschreiben Sie mit dem Finger oder der Maus.",
      ),
    ).toBeInTheDocument();
  });

  it("blocks agreeing without a signature and sends the confirmed one", async () => {
    const submit = vi
      .spyOn(parentApi, "submitDeclaration")
      .mockResolvedValue(submission("agreed"));
    render(
      <NewsDetailModal item={item()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zustimmen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Ihre Antwort prüfen",
    });

    fireEvent.click(within(dialog).getByRole("button", { name: "Zustimmen" }));
    expect(
      (
        await within(dialog).findAllByText(
          "Bitte unterschreiben Sie und tippen Sie auf „Unterschrift bestätigen“.",
        )
      ).length,
    ).toBeGreaterThan(0);
    expect(submit).not.toHaveBeenCalled();

    sign(dialog);
    expect(within(dialog).getByAltText("Ihre Unterschrift")).toHaveAttribute(
      "src",
      PNG_DATA_URL,
    );
    fireEvent.click(within(dialog).getByRole("button", { name: "Zustimmen" }));

    await waitFor(() =>
      expect(submit).toHaveBeenCalledWith("42", {
        studentId: "5",
        action: "agreed",
        versionId: "12",
        password: undefined,
        signature: { method: "drawn", imagePngBase64: "iVBORw0KGgo=" },
      }),
    );
  });

  it("declines without asking for a signature", async () => {
    const submit = vi
      .spyOn(parentApi, "submitDeclaration")
      .mockResolvedValue(submission("declined"));
    render(
      <NewsDetailModal item={item()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Ihre Antwort prüfen",
    });
    expect(
      within(dialog).queryByRole("img", { name: "Feld zum Unterschreiben" }),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "moto speichert Ihre Antwort mit Datum und Uhrzeit. Die Schule sieht sie.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Ablehnen" }));

    await waitFor(() => expect(submit).toHaveBeenCalledTimes(1));
    expect(submit.mock.calls[0]?.[1]).toMatchObject({ action: "declined" });
    expect(submit.mock.calls[0]?.[1]?.signature).toBeUndefined();
  });

  it("keeps the dialog and asks for a new signature when the server refuses it", async () => {
    const submit = vi
      .spyOn(parentApi, "submitDeclaration")
      .mockRejectedValueOnce(
        new ApiError("bad png", 400, {
          code: "care.declaration_signature_invalid",
        }),
      );
    render(
      <NewsDetailModal item={item()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zustimmen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Ihre Antwort prüfen",
    });
    sign(dialog);
    fireEvent.click(within(dialog).getByRole("button", { name: "Zustimmen" }));

    expect(
      await within(dialog).findByText(
        catalogText("care.declaration_signature_invalid", "die Antwort"),
      ),
    ).toBeInTheDocument();
    expect(submit).toHaveBeenCalledTimes(1);
    expect(
      within(dialog).getByRole("img", { name: "Feld zum Unterschreiben" }),
    ).toBeInTheDocument();
  });

  it("keeps the dialog open for both signature codes", () => {
    for (const code of [
      "care.declaration_signature_required",
      "care.declaration_signature_invalid",
    ] as const) {
      expect(
        declarationErrorOutcome(new ApiError("x", 400, { code })),
      ).toMatchObject({ keepDialog: true, reload: false });
    }
  });
});
