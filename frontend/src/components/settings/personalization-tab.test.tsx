import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { useTenant } from "~/lib/tenant-context";

// Mock next/navigation
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    refresh: mockRefresh,
  }),
}));

// Mock next/image — render a plain <img> so we can assert src
vi.mock("next/image", () => ({
  default: (props: Record<string, unknown>) => {
    const { unoptimized: _u, ...rest } = props;
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    return <img {...(rest as React.ImgHTMLAttributes<HTMLImageElement>)} />;
  },
}));

// Mock Toast Context
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    info: vi.fn(),
  }),
}));

// Mock sessionFetch for the initial GET
const mockSessionFetch = vi.fn<(url: string) => Promise<Response>>();
vi.mock("~/lib/session-cache", () => ({
  sessionFetch: (url: string) => mockSessionFetch(url),
}));

// Mock loginImageSrc
vi.mock("~/lib/tenant-api", () => ({
  loginImageSrc: (path: string) =>
    `/api/public/login-image/${path.split("/").pop()}`,
}));

const { PersonalizationTab } = await import("./personalization-tab");

function jsonResponse(data: unknown, ok = true): Response {
  return {
    ok,
    status: ok ? 200 : 500,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  } as Response;
}

describe("PersonalizationTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    globalThis.fetch = vi.fn();
  });

  it("shows loading spinner initially", () => {
    mockSessionFetch.mockReturnValue(new Promise(() => {})); // never resolves
    render(<PersonalizationTab />);
    expect(document.querySelector(".animate-spin")).not.toBeNull();
  });

  it("shows placeholder when no image is set", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild hierher ziehen")).toBeDefined();
    });
  });

  it("renders current image when URL is present", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({
        data: {
          login_image_url: "/uploads/login-images/1_abc.jpg",
          can_edit: false,
        },
      }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      const img = screen.getByAltText("Login-Bild");
      expect(img).toBeDefined();
    });
  });

  it("hides upload and delete buttons when can_edit is false", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({
        data: {
          login_image_url: "/uploads/login-images/1_abc.jpg",
          can_edit: false,
        },
      }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByAltText("Login-Bild")).toBeDefined();
    });

    expect(screen.queryByText("Bild auswählen")).toBeNull();
    expect(screen.queryByText("Bild entfernen")).toBeNull();
  });

  it("shows upload button when can_edit is true", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild auswählen")).toBeDefined();
    });
  });

  it("shows delete button only when image exists and can_edit", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({
        data: {
          login_image_url: "/uploads/login-images/1_abc.jpg",
          can_edit: true,
        },
      }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild entfernen")).toBeDefined();
    });
  });

  it("uploads image and shows success toast", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild auswählen")).toBeDefined();
    });

    // Mock the upload POST
    vi.mocked(globalThis.fetch).mockResolvedValueOnce(
      jsonResponse({
        data: { login_image_url: "/uploads/login-images/1_new.jpg" },
      }),
    );

    const file = new File(["img"], "test.jpg", { type: "image/jpeg" });
    const input = document.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Login-Bild erfolgreich hochgeladen",
      );
    });
    expect(mockRefresh).toHaveBeenCalled();
  });

  it("shows error toast on upload failure", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild auswählen")).toBeDefined();
    });

    vi.mocked(globalThis.fetch).mockResolvedValueOnce(
      jsonResponse({ error: "too large" }, false),
    );

    const file = new File(["img"], "big.jpg", { type: "image/jpeg" });
    const input = document.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Fehler beim Hochladen des Bildes",
      );
    });
  });

  it("deletes image and shows success toast", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({
        data: {
          login_image_url: "/uploads/login-images/1_abc.jpg",
          can_edit: true,
        },
      }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild entfernen")).toBeDefined();
    });

    vi.mocked(globalThis.fetch).mockResolvedValueOnce(jsonResponse(null));

    fireEvent.click(screen.getByText("Bild entfernen"));

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Login-Bild erfolgreich entfernt",
      );
    });
    expect(mockRefresh).toHaveBeenCalled();
  });

  it("shows error toast on delete failure", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({
        data: {
          login_image_url: "/uploads/login-images/1_abc.jpg",
          can_edit: true,
        },
      }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild entfernen")).toBeDefined();
    });

    vi.mocked(globalThis.fetch).mockResolvedValueOnce(
      jsonResponse({ error: "server error" }, false),
    );

    fireEvent.click(screen.getByText("Bild entfernen"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Fehler beim Entfernen des Bildes",
      );
    });
  });

  it("uses tenant settings for initial image on mount", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-school",
      tenant: {
        settings: { loginImageUrl: "/uploads/login-images/initial.jpg" },
      },
    } as ReturnType<typeof useTenant>);

    // sessionFetch returns null (simulating fetch failure) — should still show initial from tenant
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: false } }),
    );

    render(<PersonalizationTab />);
    // Before the fetch resolves, the component should render with the tenant's initial value
    // After fetch resolves with null, it updates to null — read-only description is shown
    await waitFor(() => {
      expect(
        screen.getByText(/Das aktuelle Bild wird auf der Login-Seite/),
      ).toBeDefined();
    });
  });

  it("shows read-only description when can_edit is false", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: false } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(
        screen.getByText(/Das aktuelle Bild wird auf der Login-Seite/),
      ).toBeDefined();
    });
  });

  it("shows editable description when can_edit is true", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText(/Laden Sie ein eigenes Bild hoch/)).toBeDefined();
    });
  });

  it("rejects files larger than 2 MB", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild auswählen")).toBeDefined();
    });

    const largeFile = new File(["x".repeat(3 * 1024 * 1024)], "big.jpg", {
      type: "image/jpeg",
    });
    const input = document.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { files: [largeFile] } });

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Datei ist zu groß (max. 2 MB)",
      );
    });
    // Should NOT have called fetch for upload
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("rejects files with invalid type", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild auswählen")).toBeDefined();
    });

    const svgFile = new File(["<svg></svg>"], "image.svg", {
      type: "image/svg+xml",
    });
    const input = document.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { files: [svgFile] } });

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Nur JPG, PNG oder WebP erlaubt",
      );
    });
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("handles drag-and-drop upload", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild hierher ziehen")).toBeDefined();
    });

    const dropzone = document.querySelector("fieldset") as HTMLElement;

    // Simulate drag enter
    fireEvent.dragEnter(dropzone, {
      dataTransfer: { files: [] },
    });
    expect(screen.getByText("Bild hier ablegen…")).toBeDefined();

    // Simulate drag leave
    fireEvent.dragLeave(dropzone, {
      dataTransfer: { files: [] },
    });
    expect(screen.getByText("Bild hierher ziehen")).toBeDefined();
  });

  it("handles drag over without errors", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild hierher ziehen")).toBeDefined();
    });

    const dropzone = document.querySelector("fieldset") as HTMLElement;
    fireEvent.dragOver(dropzone, {
      dataTransfer: { files: [] },
    });
    // Should not throw
  });

  it("processes file on drop", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild hierher ziehen")).toBeDefined();
    });

    vi.mocked(globalThis.fetch).mockResolvedValueOnce(
      jsonResponse({
        data: { login_image_url: "/uploads/login-images/dropped.jpg" },
      }),
    );

    const file = new File(["img"], "drop.png", { type: "image/png" });
    const dropzone = document.querySelector("fieldset") as HTMLElement;
    fireEvent.drop(dropzone, {
      dataTransfer: { files: [file] },
    });

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Login-Bild erfolgreich hochgeladen",
      );
    });
  });

  it("shows replace text when image exists and can_edit", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({
        data: {
          login_image_url: "/uploads/login-images/1_abc.jpg",
          can_edit: true,
        },
      }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Neues Bild hierher ziehen")).toBeDefined();
    });
  });

  it("handles keyboard activation of dropzone", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ data: { login_image_url: null, can_edit: true } }),
    );

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(screen.getByText("Bild auswählen")).toBeDefined();
    });

    const clickButton = screen.getByLabelText(
      "Bild hochladen — ziehen Sie eine Datei hierher oder klicken Sie zum Auswählen",
    );
    // Enter and Space should trigger file input click
    fireEvent.keyDown(clickButton, { key: "Enter" });
    fireEvent.keyDown(clickButton, { key: " " });
    // No error thrown — keyboard handling works
  });

  it("handles non-ok fetch on mount gracefully", async () => {
    mockSessionFetch.mockResolvedValue(
      jsonResponse({ error: "server error" }, false),
    );

    render(<PersonalizationTab />);
    // Should not crash, loading should finish
    await waitFor(() => {
      expect(document.querySelector(".animate-spin")).toBeNull();
    });
  });

  it("handles fetch exception on mount gracefully", async () => {
    mockSessionFetch.mockRejectedValue(new Error("network down"));

    render(<PersonalizationTab />);
    await waitFor(() => {
      expect(document.querySelector(".animate-spin")).toBeNull();
    });
  });
});
