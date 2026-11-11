import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { render as rtlRender, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  TraegerCatalogPreview,
  TraegerPhaseDetail,
} from "~/lib/traeger-catalog-api";

const { mockPreview, mockRollout, mockFetchPhase } = vi.hoisted(() => ({
  mockPreview: vi.fn(),
  mockRollout: vi.fn(),
  mockFetchPhase: vi.fn(),
}));

vi.mock("~/lib/traeger-catalog-api", () => ({
  previewTraegerPhase: (id: string) => mockPreview(id),
  rolloutTraegerPhase: (id: string, fingerprint: string) =>
    mockRollout(id, fingerprint),
  fetchTraegerPhase: (id: string) => mockFetchPhase(id),
  fetchTraegerPhases: () => Promise.resolve([]),
}));
vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerSchools: () =>
    Promise.resolve([
      { id: "11", name: "OGS Sonnenhof", slug: "a", subdomain: "a", city: "" },
      { id: "12", name: "Demo-Schule", slug: "b", subdomain: "b", city: "" },
    ]),
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", scope: "org", orgId: 7 } },
    status: "authenticated",
  }),
}));

import TraegerCatalogPreviewPage from "./page";

function render(ui: React.ReactElement) {
  return rtlRender(
    <ToastProvider>
      <SWRConfig value={{ provider: () => new Map() }}>{ui}</SWRConfig>
    </ToastProvider>,
  );
}

const preview: TraegerCatalogPreview = {
  traegerPhaseId: "7",
  version: 2,
  fingerprint: "fp-1",
  schools: [
    {
      schoolId: "11",
      action: "update",
      outcome: "ready",
      errorCode: null,
      phase: [{ field: "name", state: "unchanged", traeger: "A", school: "A" }],
      offerings: [
        {
          traegerOfferingId: "31",
          name: "Frühbetreuung",
          action: "update",
          fields: [
            { field: "capacity", state: "keeps", traeger: 20, school: 12 },
          ],
          pricesFrom: "2027-09-01",
          contractTemplate: true,
        },
      ],
    },
    {
      schoolId: "12",
      action: "create",
      outcome: "rejected",
      errorCode: "enrollment.care_offering_pickup_times_required",
      phase: [],
      offerings: [],
    },
  ],
};

describe("TraegerCatalogPreviewPage (#4704)", () => {
  beforeEach(() => {
    mockPreview.mockReset();
    mockRollout.mockReset();
    mockFetchPhase.mockReset();
    mockFetchPhase.mockResolvedValue({
      id: "7",
      draft: { name: "Schuljahr 2027/28" },
    } as unknown as TraegerPhaseDetail);
  });

  it("shows per school what happens, the value a school keeps and a refusal with its reason", async () => {
    mockPreview.mockResolvedValue(preview);
    render(<TraegerCatalogPreviewPage />);

    const keeps = await screen.findByText(
      "Die Schule hat geändert – bleibt so",
    );
    const row = keeps.closest("tr");
    expect(row).not.toBeNull();
    expect(within(row!).getByText("12")).toBeInTheDocument();
    expect(within(row!).getByText("20")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Neue Preise ab September 2027. Die Vertragsvorlage wird übernommen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Lehnt ab")).toBeInTheDocument();
    expect(
      screen.getByText(
        /Bitte tragen Sie für jeden Betreuungstag eine Gehzeit ein\. Diese Schule behält ihren bisherigen Stand\./,
      ),
    ).toBeInTheDocument();
  });

  it("rolls out with the preview's fingerprint and shows the result per school", async () => {
    mockPreview.mockResolvedValue(preview);
    mockRollout.mockResolvedValue({
      traegerPhaseId: "7",
      version: 2,
      schools: [
        { schoolId: "11", outcome: "applied", errorCode: null },
        { schoolId: "12", outcome: "stale", errorCode: null },
      ],
    });
    render(<TraegerCatalogPreviewPage />);

    await userEvent.click(
      await screen.findByRole("button", { name: "Freigeben und übertragen" }),
    );

    expect(mockRollout).toHaveBeenCalledWith("7", "fp-1");
    const result = await screen.findByRole("table");
    expect(within(result).getByText("Übernommen")).toBeInTheDocument();
    expect(within(result).getByText("Nicht übertragen")).toBeInTheDocument();
    expect(
      screen.queryByText("Die Schule hat geändert – bleibt so"),
    ).not.toBeInTheDocument();
  });

  it("loads the preview again when something changed since the preview", async () => {
    mockPreview
      .mockResolvedValueOnce(preview)
      .mockResolvedValueOnce({ ...preview, fingerprint: "fp-2" });
    mockRollout.mockRejectedValue(
      new ApiError("stale", 409, { code: "enrollment.traeger_preview_stale" }),
    );
    render(<TraegerCatalogPreviewPage />);

    await userEvent.click(
      await screen.findByRole("button", { name: "Freigeben und übertragen" }),
    );

    await vi.waitFor(() => expect(mockPreview).toHaveBeenCalledTimes(2));
    expect(
      await screen.findByText(
        "Seit der Vorschau hat sich etwas geändert. Bitte sehen Sie sich die Vorschau noch einmal an.",
      ),
    ).toBeInTheDocument();
  });
});
