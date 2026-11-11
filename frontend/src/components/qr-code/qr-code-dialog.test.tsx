import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { QrLogo } from "~/lib/qr-code/qr-image";
import { QrCodeDialog } from "./qr-code-dialog";

const { useQrLogo, brandedQrPng, downloadBlob } = vi.hoisted(() => ({
  useQrLogo: vi.fn(),
  brandedQrPng: vi.fn<() => Promise<Blob>>(),
  downloadBlob: vi.fn(),
}));

// The canvas work runs in a real browser only; the dialog is tested around it.
vi.mock("./use-qr-logo", () => ({ useQrLogo }));
vi.mock("~/lib/qr-code/qr-image", () => ({ brandedQrPng }));
vi.mock("~/lib/file-download", () => ({ downloadBlob }));

const URL_VALUE = "https://ogs-test.moto-app.de/anmeldung";
const SCHOOL_LOGO = "/api/public/login-image/logo.png";

function logo(name: string, hasPlainBackground = false): QrLogo {
  return {
    dataUrl: `data:image/png;base64,${name}`,
    aspect: 2,
    hasPlainBackground,
  };
}

/** Answers per source: a stored address, the moto logo, or a chosen file. */
function logos(
  answers: Readonly<{ school?: QrLogo | "failed"; custom?: QrLogo | "failed" }>,
) {
  useQrLogo.mockImplementation((source: string | Blob | null) => {
    if (source === null) return { status: "loading", logo: null };
    let answer: QrLogo | "failed" | undefined = logo("moto");
    if (source === SCHOOL_LOGO) answer = answers.school;
    if (typeof source !== "string") answer = answers.custom;
    if (answer === undefined || answer === "failed") {
      return { status: "failed", logo: null };
    }
    return { status: "ready", logo: answer };
  });
}

function renderDialog(withSchoolLogo: boolean) {
  return render(
    <QrCodeDialog
      isOpen
      onClose={vi.fn()}
      url={URL_VALUE}
      title="QR-Code für die Anmeldung"
      fileName="qr-code-anmeldung"
      brandLogo={
        withSchoolLogo ? { src: SCHOOL_LOGO, label: "Schule" } : undefined
      }
    />,
  );
}

function shownLogo(): string {
  const src = screen
    .getByRole("img", { name: `QR-Code. Er führt zu ${URL_VALUE}` })
    .getAttribute("src")!;
  return decodeURIComponent(src);
}

const pressed = (name: string) =>
  screen.getByRole("button", { name }).getAttribute("aria-pressed");

describe("QrCodeDialog", () => {
  beforeEach(() => {
    useQrLogo.mockReset();
    brandedQrPng.mockReset();
    downloadBlob.mockReset();
  });

  it("starts with the stored logo when the school has one", () => {
    logos({ school: logo("schule") });
    renderDialog(true);

    expect(pressed("Schule")).toBe("true");
    expect(shownLogo()).toContain("base64,schule");

    fireEvent.click(screen.getByRole("button", { name: "moto" }));

    expect(shownLogo()).toContain("base64,moto");
  });

  it("starts with the moto logo and offers no school choice without a stored logo", () => {
    logos({ school: "failed" });
    renderDialog(true);

    expect(pressed("moto")).toBe("true");
    expect(shownLogo()).toContain("base64,moto");
    expect(screen.queryByRole("button", { name: "Schule" })).toBeNull();
  });

  it("takes an own picture and lets the person keep its background", () => {
    logos({ custom: logo("eigenes", true) });
    renderDialog(false);

    fireEvent.click(screen.getByRole("button", { name: "Eigenes Bild" }));
    fireEvent.change(screen.getByLabelText("Bild auswählen"), {
      target: {
        files: [new File(["x"], "wappen.png", { type: "image/png" })],
      },
    });

    expect(shownLogo()).toContain("base64,eigenes");
    const toggle = screen.getByRole("switch", {
      name: "Hintergrund entfernen",
    });
    expect(toggle.getAttribute("aria-checked")).toBe("true");

    fireEvent.click(toggle);

    expect(useQrLogo).toHaveBeenLastCalledWith(expect.any(File), false);
  });

  it("refuses a file that is no picture and says which formats work", () => {
    logos({});
    renderDialog(false);

    fireEvent.click(screen.getByRole("button", { name: "Eigenes Bild" }));
    fireEvent.change(screen.getByLabelText("Bild auswählen"), {
      target: {
        files: [new File(["x"], "brief.pdf", { type: "application/pdf" })],
      },
    });

    expect(
      screen.getByText(
        "Bitte wählen Sie ein Bild im Format PNG, JPG oder WebP.",
      ),
    ).toBeInTheDocument();
    // Still the code without a logo of its own: the file was not taken.
    expect(shownLogo().includes("<image")).toBe(false);
  });

  it("saves the picture and the print file under the given name", async () => {
    logos({});
    const png = new Blob(["png"], { type: "image/png" });
    brandedQrPng.mockResolvedValue(png);
    renderDialog(false);

    fireEvent.click(screen.getByRole("button", { name: "Bild speichern" }));
    await waitFor(() =>
      expect(downloadBlob).toHaveBeenCalledWith(png, "qr-code-anmeldung.png"),
    );
    expect(brandedQrPng).toHaveBeenCalledWith(
      expect.objectContaining({ size: expect.any(Number) }),
      { logoDataUrl: "data:image/png;base64,moto", pixelSize: 2048 },
    );

    fireEvent.click(screen.getByRole("button", { name: "Druckdatei (SVG)" }));
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledTimes(2));
    const [svg, name] = downloadBlob.mock.calls[1] as [Blob, string];
    expect(name).toBe("qr-code-anmeldung.svg");
    expect(svg.type).toBe("image/svg+xml");
  });

  it("says so when saving fails", async () => {
    logos({});
    brandedQrPng.mockRejectedValue(new Error("png_failed"));
    renderDialog(false);

    fireEvent.click(screen.getByRole("button", { name: "Bild speichern" }));

    expect(
      await screen.findByText(
        "Der QR-Code konnte nicht gespeichert werden. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(downloadBlob).not.toHaveBeenCalled();
  });
});
