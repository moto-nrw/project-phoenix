import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import { EnrollmentRequestUploadList } from "./admin-enrollment-upload-list";

const value = [
  {
    upload_id: "11",
    filename: "bescheid.pdf",
    size_bytes: 2048,
    content_type: "application/pdf",
  },
  {
    upload_id: "12",
    filename: "alt.png",
    size_bytes: 1024,
    content_type: "image/png",
  },
];
const uploads = [
  {
    id: "11",
    field_key: "einkommen_nachweis",
    filename: "bescheid.pdf",
    size_bytes: 2048,
    content_type: "application/pdf",
    uploaded_at: "2026-09-01T08:00:00Z",
  },
];

describe("EnrollmentRequestUploadList", () => {
  it("opens a stored file through the proxy and marks a deleted one", () => {
    render(
      <EnrollmentRequestUploadList
        requestId="5"
        value={value}
        uploads={uploads}
        canView
      />,
    );
    const link = screen.getByRole("link", { name: /Öffnen/ });
    expect(link).toHaveAttribute(
      "href",
      "/api/enrollment/admin/requests/5/uploads/11",
    );
    expect(link).toHaveAttribute("target", "_blank");
    expect(screen.getByText("alt.png")).toBeInTheDocument();
    expect(
      screen.getByText("Nachweis gelöscht (Löschfrist abgelaufen)"),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("link")).toHaveLength(1);
  });

  it("names the missing permission instead of a link", () => {
    render(
      <EnrollmentRequestUploadList
        requestId="5"
        value={value.slice(0, 1)}
        uploads={uploads}
        canView={false}
      />,
    );
    expect(screen.getByText("bescheid.pdf")).toBeInTheDocument();
    expect(
      screen.getByText("Keine Berechtigung zum Öffnen"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
