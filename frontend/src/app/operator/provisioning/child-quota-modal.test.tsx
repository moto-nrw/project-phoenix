import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockUpdateSchool, mockLoggerError } = vi.hoisted(() => ({
  mockUpdateSchool: vi.fn(),
  mockLoggerError: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    updateSchool: (...args: unknown[]) => mockUpdateSchool(...args),
  },
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: mockLoggerError,
  }),
}));

import { ChildQuotaModal } from "./child-quota-modal";
import { OperatorApiError } from "~/lib/operator/api-helpers";
import type { SchoolSummary } from "~/lib/operator/provisioning-helpers";

const school: SchoolSummary = {
  id: "10",
  organizationId: "1",
  organizationName: "Test Org",
  name: "OGS Am Park",
  slug: "am-park",
  subdomain: "am-park",
  active: true,
  hidden: false,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  deletedAt: null,
  address: "Parkweg 1",
  city: "Köln",
  zip: "50667",
  phone: "",
  email: "",
  kontenCount: 2,
  geraeteCount: 1,
  personenCount: 30,
  childQuotaBundles: 2,
  childQuotaBundleSize: 50,
  childQuotaCount: 80,
};

const unlimited: SchoolSummary = {
  ...school,
  childQuotaBundles: null,
  childQuotaBundleSize: 50,
};

function renderModal(
  target: SchoolSummary = school,
  current: SchoolSummary = target,
) {
  const onClose = vi.fn();
  const onUpdated = vi.fn().mockResolvedValue(undefined);
  const loadCurrentSchool = vi.fn().mockResolvedValue(current);
  render(
    <ChildQuotaModal
      isOpen
      onClose={onClose}
      school={target}
      loadCurrentSchool={loadCurrentSchool}
      onUpdated={onUpdated}
    />,
  );
  return { onClose, onUpdated, loadCurrentSchool };
}

function schoolFieldsOf(target: SchoolSummary) {
  return {
    organization_id: 1,
    name: target.name,
    slug: target.slug,
    subdomain: target.subdomain,
    address: target.address,
    city: target.city,
    zip: target.zip,
    phone: target.phone,
    email: target.email,
    active: target.active,
    hidden: target.hidden,
  };
}

describe("ChildQuotaModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdateSchool.mockResolvedValue({});
  });

  it("shows the Kinderkontingent next to the Kontingentzahl", () => {
    renderModal();

    expect(screen.getByLabelText("Anzahl Bundles")).toHaveValue("2");
    expect(screen.getByLabelText("Kinder pro Bundle")).toHaveValue("50");
    expect(screen.getByTestId("child-quota-limit")).toHaveTextContent(
      "100 Kinder",
    );
    expect(screen.getByTestId("child-quota-count")).toHaveTextContent(
      "80 Kinder",
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("prefills 50 children per bundle for a school without Kinderkontingent", () => {
    renderModal(unlimited);

    expect(screen.getByLabelText("Anzahl Bundles")).toHaveValue("");
    expect(screen.getByLabelText("Kinder pro Bundle")).toHaveValue("50");
    expect(screen.getByTestId("child-quota-limit")).toHaveTextContent("–");
    expect(
      screen.queryByRole("button", { name: "Entfernen" }),
    ).not.toBeInTheDocument();
  });

  it("preserves the stored bundle size when re-enabling a Kinderkontingent", () => {
    renderModal({ ...unlimited, childQuotaBundleSize: 40 });

    expect(screen.getByLabelText("Kinder pro Bundle")).toHaveValue("40");
  });

  it("recomputes the Kinderkontingent while typing and saves whole bundles", async () => {
    const { onClose, onUpdated } = renderModal(unlimited);

    fireEvent.change(screen.getByLabelText("Anzahl Bundles"), {
      target: { value: "3" },
    });
    expect(screen.getByTestId("child-quota-limit")).toHaveTextContent(
      "150 Kinder",
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockUpdateSchool).toHaveBeenCalledWith("10", {
      ...schoolFieldsOf(unlimited),
      child_quota: { bundles: 3, bundle_size: 50 },
    });
    expect(onUpdated).toHaveBeenCalled();
  });

  it("warns when the Kinderkontingent drops below the Kontingentzahl and still saves", async () => {
    const { onClose } = renderModal({
      ...school,
      childQuotaBundles: 3,
      childQuotaCount: 120,
    });

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Anzahl Bundles"), {
      target: { value: "2" },
    });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "120 Kinder zählen, Kontingent 100.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockUpdateSchool).toHaveBeenCalledWith(
      "10",
      expect.objectContaining({
        child_quota: { bundles: 2, bundle_size: 50 },
      }),
    );
  });

  it("shows a form error for a partial bundle and saves nothing", () => {
    const { onClose } = renderModal();

    fireEvent.change(screen.getByLabelText("Anzahl Bundles"), {
      target: { value: "1,5" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(screen.getByLabelText("Anzahl Bundles")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(
      screen.getByText("Bitte geben Sie eine ganze Zahl von 1 bis 1000 ein."),
    ).toBeInTheDocument();
    expect(mockUpdateSchool).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("removes the Kinderkontingent only after the confirmation", async () => {
    const { onClose, onUpdated } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));
    expect(
      screen.getByRole("heading", { name: "Kinderkontingent entfernen" }),
    ).toBeInTheDocument();
    expect(mockUpdateSchool).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));
    expect(mockUpdateSchool).not.toHaveBeenCalled();
    fireEvent.click(
      screen.getByRole("button", { name: "Kinderkontingent entfernen" }),
    );

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockUpdateSchool).toHaveBeenCalledWith("10", {
      ...schoolFieldsOf(school),
      child_quota: null,
    });
    expect(onUpdated).toHaveBeenCalled();
  });

  it("goes back to the form when the removal is cancelled", () => {
    renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(screen.getByLabelText("Anzahl Bundles")).toHaveValue("2");
    expect(mockUpdateSchool).not.toHaveBeenCalled();
  });

  it("saves on the freshly loaded school so a concurrent edit survives", async () => {
    const renamed = { ...school, name: "OGS Am Park (neu)", active: false };
    const { onClose, loadCurrentSchool } = renderModal(school, renamed);

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(loadCurrentSchool).toHaveBeenCalledTimes(1);
    expect(mockUpdateSchool).toHaveBeenCalledWith("10", {
      ...schoolFieldsOf(renamed),
      child_quota: { bundles: 2, bundle_size: 50 },
    });
  });

  it("does not save from stale school data when refreshing the school fails", async () => {
    const { onClose, loadCurrentSchool } = renderModal();
    loadCurrentSchool.mockRejectedValue(new Error("refresh failed"));

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Das Kinderkontingent wurde nicht gespeichert. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(mockUpdateSchool).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes after saving when the subsequent refresh fails", async () => {
    const { onClose, onUpdated } = renderModal();
    onUpdated.mockRejectedValue(new Error("refresh failed"));

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockUpdateSchool).toHaveBeenCalled();
    expect(
      screen.queryByText(
        "Das Kinderkontingent wurde nicht gespeichert. Bitte versuchen Sie es erneut.",
      ),
    ).not.toBeInTheDocument();
    expect(mockLoggerError).toHaveBeenCalledWith(
      "child_quota_refresh_failed",
      expect.objectContaining({ school_id: "10" }),
    );
  });

  it("asks for a reload when the school changed underneath", async () => {
    mockUpdateSchool.mockRejectedValue(new OperatorApiError("conflict", 409));
    const { onClose } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Schule wurde inzwischen geändert. Bitte laden Sie die Seite neu.",
      ),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("keeps the dialog open with an error when the server refuses", async () => {
    mockUpdateSchool.mockRejectedValue(
      new OperatorApiError(
        "child quota bundles must be between 1 and 1000",
        400,
      ),
    );
    const { onClose } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Das Kinderkontingent wurde nicht gespeichert. Bitte prüfen Sie die Eingaben.",
      ),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(mockLoggerError).toHaveBeenCalledWith(
      "child_quota_update_failed",
      expect.objectContaining({ school_id: "10" }),
    );
  });
});
