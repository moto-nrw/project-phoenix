import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ModalProvider } from "~/components/dashboard/modal-context";
import {
  CareRequestApiError,
  fetchCareScheduleChangeRequest,
  type StaffCareRequestDetail,
} from "~/lib/care-request-review-api";
import { PickupRequestDetailModal } from "./pickup-request-detail-modal";

vi.mock("~/lib/care-request-review-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/care-request-review-api")
  >("~/lib/care-request-review-api");
  return { ...actual, fetchCareScheduleChangeRequest: vi.fn() };
});

const mockFetch = vi.mocked(fetchCareScheduleChangeRequest);

function detail(
  overrides: Partial<StaffCareRequestDetail> = {},
): StaffCareRequestDetail {
  return {
    id: "42",
    student_id: "7",
    first_name: "Mia",
    last_name: "Muster",
    status: "approved",
    request_kind: "pickup_change",
    requested: [{ label: "15.09.2026 · Abholzeit", old: "", new: "14:30" }],
    request_reason: "Arzttermin",
    decision_reason: "Passt",
    created_at: "2026-09-08T18:40:00Z",
    decided_at: "2026-09-09T04:22:00Z",
    decided_by_name: "Paula Planerin",
    pickup_change: {
      date: "2026-09-15",
      pickup_time: "14:30",
      previous_pickup_time: "15:30",
    },
    ...overrides,
  };
}

function Wrapper({ children }: { readonly children: ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

describe("PickupRequestDetailModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("zeigt Kind, Tag, Uhrzeiten, Grund und Entscheidung einer freigegebenen Anfrage", async () => {
    mockFetch.mockResolvedValue(detail());
    render(<PickupRequestDetailModal requestId="42" onClose={vi.fn()} />, {
      wrapper: Wrapper,
    });

    expect(await screen.findByText("Mia Muster")).toBeInTheDocument();
    expect(mockFetch).toHaveBeenCalledWith("42");
    expect(screen.getByText(/15\.09\.2026/)).toBeInTheDocument();
    expect(screen.getByText("14:30 Uhr")).toBeInTheDocument();
    expect(screen.getByText("15:30 Uhr")).toBeInTheDocument();
    expect(screen.getByText("Arzttermin")).toBeInTheDocument();
    expect(screen.getByText("Freigegeben")).toBeInTheDocument();
    expect(screen.getByText("Paula Planerin")).toBeInTheDocument();
    expect(screen.getByText("Passt")).toBeInTheDocument();
    // Eine entschiedene Anfrage bietet keinen Weg in die Arbeitsliste.
    expect(
      screen.queryByRole("button", { name: "Zu den Anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("bleibt bei abgelehnten und zurückgezogenen Anfragen lesbar", async () => {
    mockFetch.mockResolvedValue(
      detail({ status: "rejected", decision_reason: "Kein Personal" }),
    );
    const { unmount } = render(
      <PickupRequestDetailModal requestId="42" onClose={vi.fn()} />,
      { wrapper: Wrapper },
    );
    expect(await screen.findByText("Abgelehnt")).toBeInTheDocument();
    expect(screen.getByText("Kein Personal")).toBeInTheDocument();
    unmount();

    mockFetch.mockResolvedValue(
      detail({
        status: "withdrawn",
        decision_reason: undefined,
        decided_by_name: undefined,
      }),
    );
    render(<PickupRequestDetailModal requestId="42" onClose={vi.fn()} />, {
      wrapper: Wrapper,
    });
    expect(await screen.findByText("Zurückgezogen")).toBeInTheDocument();
    expect(
      screen.getByText(/Die Eltern haben die Anfrage zurückgezogen/),
    ).toBeInTheDocument();
  });

  it("verweist bei einer offenen Anfrage auf die Anfragen-Seite statt hier zu entscheiden", async () => {
    mockFetch.mockResolvedValue(
      detail({
        status: "pending",
        decided_at: undefined,
        decided_by_name: undefined,
        decision_reason: undefined,
      }),
    );
    const onOpenQueue = vi.fn();
    render(
      <PickupRequestDetailModal
        requestId="42"
        onClose={vi.fn()}
        onOpenQueue={onOpenQueue}
      />,
      { wrapper: Wrapper },
    );

    expect(await screen.findByText("Offen")).toBeInTheDocument();
    expect(screen.getByText(/Noch nicht entschieden/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Freigeben|Ablehnen/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Zu den Anfragen" }));
    expect(onOpenQueue).toHaveBeenCalledTimes(1);
  });

  it("erklärt eine entfernte Anfrage und fehlenden Zugriff verständlich", async () => {
    mockFetch.mockRejectedValue(
      new CareRequestApiError("not found", undefined, 404),
    );
    const { unmount } = render(
      <PickupRequestDetailModal requestId="42" onClose={vi.fn()} />,
      { wrapper: Wrapper },
    );
    expect(
      await screen.findByText(/Diese Anfrage gibt es nicht mehr/),
    ).toBeInTheDocument();
    unmount();

    mockFetch.mockRejectedValue(
      new CareRequestApiError("forbidden", undefined, 403),
    );
    render(<PickupRequestDetailModal requestId="42" onClose={vi.fn()} />, {
      wrapper: Wrapper,
    });
    expect(
      await screen.findByText(/dürfen Sie keine Anfragen ansehen/),
    ).toBeInTheDocument();
  });

  it("meldet einen Ladefehler und schließt zurück zur Unterhaltung", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    mockFetch.mockRejectedValue(new Error("network"));
    const onClose = vi.fn();
    render(<PickupRequestDetailModal requestId="42" onClose={onClose} />, {
      wrapper: Wrapper,
    });
    expect(
      await screen.findByText(/konnte nicht geladen werden/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Schließen" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    consoleError.mockRestore();
  });

  it("lädt nichts, solange keine Anfrage gewählt ist", () => {
    render(<PickupRequestDetailModal requestId={null} onClose={vi.fn()} />, {
      wrapper: Wrapper,
    });
    expect(mockFetch).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
