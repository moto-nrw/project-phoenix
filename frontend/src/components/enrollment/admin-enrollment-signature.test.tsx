import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AdminEnrollmentSignature } from "./admin-enrollment-signature";
import { downloadAdminEnrollmentSignatureProof } from "~/lib/enrollment-admin-api";
import type { EnrollmentSignature } from "~/lib/enrollment-submission-api";

const showError = vi.hoisted(() => vi.fn());
vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => ({ show: showError }),
}));
vi.mock("~/lib/enrollment-admin-api", () => ({
  downloadAdminEnrollmentSignatureProof: vi.fn(),
}));

function signature(
  overrides: Partial<EnrollmentSignature> = {},
): EnrollmentSignature {
  return {
    method: "drawn",
    signer_name: "Mara Muster",
    signed_at: "2026-09-09T08:30:00Z",
    document_sha256: "ab".repeat(32),
    integrity_ok: true,
    image_png: "iVBORw0KGgo=",
    ...overrides,
  };
}

// The staff detail of a signed enrollment (#4786).
describe("AdminEnrollmentSignature", () => {
  it("shows the image, the signer and the checksum and downloads the proof", async () => {
    vi.mocked(downloadAdminEnrollmentSignatureProof).mockResolvedValueOnce();
    render(<AdminEnrollmentSignature requestId="12" signature={signature()} />);

    expect(
      screen.getByRole("img", { name: "Unterschrift von Mara Muster" }),
    ).toHaveAttribute("src", expect.stringContaining("iVBORw0KGgo="));
    expect(screen.getByText("Mara Muster")).toBeInTheDocument();
    expect(screen.getByText("ab".repeat(32))).toBeInTheDocument();
    expect(
      screen.getByText(
        "Diese Anmeldung trägt eine einfache elektronische Signatur.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Mit Finger, Stift oder Maus gezeichnet"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Nachweis als PDF" }));
    await waitFor(() => {
      expect(downloadAdminEnrollmentSignatureProof).toHaveBeenCalledWith("12");
    });
  });

  it("warns when the stored entry no longer matches its checksum", () => {
    render(
      <AdminEnrollmentSignature
        requestId="12"
        signature={signature({
          integrity_ok: false,
          method: "typed",
          typed_name: "Mara Muster",
        })}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Der gespeicherte Eintrag wurde nachträglich verändert.",
    );
    expect(screen.getByText("Name getippt: Mara Muster")).toBeInTheDocument();
  });

  it("names the proof when the download fails", async () => {
    const failure = new Error("offline");
    vi.mocked(downloadAdminEnrollmentSignatureProof).mockRejectedValueOnce(
      failure,
    );
    render(<AdminEnrollmentSignature requestId="12" signature={signature()} />);
    fireEvent.click(screen.getByRole("button", { name: "Nachweis als PDF" }));
    await waitFor(() => {
      expect(showError).toHaveBeenCalledWith(
        failure,
        expect.objectContaining({ object: "Der Anmeldenachweis" }),
      );
    });
  });
});
