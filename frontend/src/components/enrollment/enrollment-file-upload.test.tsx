import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import deMessages from "~/i18n/messages/de.generated";
import type { FormField } from "~/lib/enrollment-form-schema-api";
import {
  effectiveFileFieldOptions,
  type UploadValue,
} from "~/lib/enrollment-uploads";
import {
  EnrollmentUploadScope,
  FileUploadInput,
  fileValueMissing,
  useEnrollmentUploadTarget,
} from "./enrollment-file-upload";

const messages = deMessages.enrollmentForm as Record<string, unknown>;

function tr(key: string, values?: Record<string, string | number | Date>) {
  const text = key
    .split(".")
    .reduce<unknown>(
      (node, part) =>
        node && typeof node === "object"
          ? (node as Record<string, unknown>)[part]
          : undefined,
      messages,
    );
  if (typeof text !== "string") return key;
  return Object.entries(values ?? {}).reduce(
    (out, [name, value]) => out.replaceAll(`{${name}}`, String(value)),
    text,
  );
}

const fileField: FormField = {
  key: "einkommen_nachweis",
  label: "Einkommensnachweis",
  type: "file",
  required: true,
  sort_order: 0,
  applies_to_child: false,
  file_options: {
    allowed_types: ["pdf", "png"],
    max_size_mb: 1,
    multiple: true,
  },
};

const changes: UploadValue[][] = [];
const NO_FILES: UploadValue[] = [];
const pendingCounts: number[] = [];

function Harness({
  field = fileField,
  phaseId = "7",
  previewMode = false,
  lateInviteToken,
  statusToken,
  uploadEndpoint,
  initial = NO_FILES,
}: Readonly<{
  field?: FormField;
  phaseId?: string;
  previewMode?: boolean;
  lateInviteToken?: string;
  statusToken?: string;
  uploadEndpoint?: string;
  initial?: UploadValue[];
}>) {
  const [value, setValue] = useState<UploadValue[]>(initial);
  const { pendingUploads, uploadTarget } = useEnrollmentUploadTarget({
    tenantSlug: "school-a",
    phaseId,
    lateInviteToken,
    statusToken,
    uploadEndpoint,
    locale: "de",
    previewMode,
  });
  pendingCounts.push(pendingUploads);
  return (
    <EnrollmentUploadScope target={uploadTarget}>
      <FileUploadInput
        field={field}
        inputName="custom_einkommen_nachweis"
        value={value}
        onChange={(next) => {
          changes.push(next);
          setValue(next);
        }}
        tr={tr}
      />
    </EnrollmentUploadScope>
  );
}

function pick(container: HTMLElement, files: File[]) {
  const input = container.querySelector<HTMLInputElement>('input[type="file"]');
  if (!input) throw new Error("file input missing");
  fireEvent.change(input, { target: { files } });
  return input;
}

afterEach(() => {
  vi.unstubAllGlobals();
  changes.length = 0;
  pendingCounts.length = 0;
});

describe("FileUploadInput", () => {
  it("uses default file kinds when an older schema has no usable types", () => {
    expect(effectiveFileFieldOptions(undefined).allowed_types).toEqual([
      "pdf",
      "jpg",
      "png",
    ]);
    expect(
      effectiveFileFieldOptions({ allowed_types: ["docx" as "pdf"] })
        .allowed_types,
    ).toEqual(["pdf", "jpg", "png"]);
  });
  it("uploads a picked file at once and lists it with its token", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "success",
          data: {
            token: "tok-1",
            filename: "bescheid.pdf",
            size_bytes: 2048,
            content_type: "application/pdf",
          },
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
    const { container } = render(<Harness />);

    pick(container, [
      new File(["%PDF-1.7"], "bescheid.pdf", { type: "application/pdf" }),
    ]);

    // While it runs, the file shows as pending.
    expect(screen.getByText("Wird hochgeladen …")).toBeInTheDocument();
    await waitFor(() => expect(changes).toHaveLength(1));
    expect(screen.getByText("bescheid.pdf")).toBeInTheDocument();
    expect(screen.getByText("2 KB")).toBeInTheDocument();
    expect(changes.at(-1)).toEqual([
      {
        token: "tok-1",
        filename: "bescheid.pdf",
        size_bytes: 2048,
        content_type: "application/pdf",
      },
    ]);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/enrollment/school-a/uploads");
    expect(init.method).toBe("POST");
    const body = init.body as FormData;
    expect(body.get("phase_id")).toBe("7");
    expect(body.get("field_key")).toBe("einkommen_nachweis");
    expect((body.get("file") as File).name).toBe("bescheid.pdf");
    // The form counted the upload while it ran and released it afterwards.
    expect(pendingCounts).toContain(1);
    await waitFor(() => expect(pendingCounts.at(-1)).toBe(0));
  });

  it("keeps selection order when concurrent uploads finish backwards", async () => {
    const resolvers: Array<(response: Response) => void> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(
        () =>
          new Promise<Response>((resolve) => {
            resolvers.push(resolve);
          }),
      ),
    );
    const { container } = render(
      <Harness
        initial={[
          {
            upload_id: "55",
            filename: "stored.pdf",
            size_bytes: 1,
            content_type: "application/pdf",
          },
        ]}
      />,
    );
    pick(
      container,
      [1, 2, 3].map(
        (n) => new File(["pdf"], `proof-${n}.pdf`, { type: "application/pdf" }),
      ),
    );
    expect(resolvers).toHaveLength(3);
    for (const n of [3, 2, 1]) {
      await act(async () => {
        resolvers[n - 1]!(
          new Response(
            JSON.stringify({
              status: "success",
              data: {
                token: `token-${n}`,
                filename: `proof-${n}.pdf`,
                size_bytes: 3,
                content_type: "application/pdf",
              },
            }),
            { status: 201, headers: { "content-type": "application/json" } },
          ),
        );
      });
    }
    expect(changes.at(-1)?.map((file) => file.filename)).toEqual([
      "stored.pdf",
      "proof-1.pdf",
      "proof-2.pdf",
      "proof-3.pdf",
    ]);
    expect(
      screen
        .getAllByRole("listitem")
        .map((item) => item.querySelector("span")?.textContent),
    ).toEqual(["stored.pdf", "proof-1.pdf", "proof-2.pdf", "proof-3.pdf"]);
    expect(pendingCounts.at(-1)).toBe(0);
  });

  it("rejects a file above the size limit without uploading it", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const { container } = render(<Harness />);

    pick(container, [
      new File([new Uint8Array(1024 * 1024 + 1)], "gross.pdf", {
        type: "application/pdf",
      }),
    ]);

    expect(
      screen.getByText("Diese Datei ist zu groß. Erlaubt sind höchstens 1 MB."),
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("forwards a status token with the upload", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(
          JSON.stringify({ status: "success", data: { token: "tok-1" } }),
          { status: 201, headers: { "content-type": "application/json" } },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);
    const { container } = render(<Harness statusToken="status-token" />);

    pick(container, [new File(["%PDF-1.7"], "bescheid.pdf")]);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect((init.body as FormData).get("status_token")).toBe("status-token");
  });

  it("uses the parent upload endpoint when the form provides one", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(
          JSON.stringify({ status: "success", data: { token: "tok-1" } }),
          { status: 201, headers: { "content-type": "application/json" } },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);
    const { container } = render(
      <Harness uploadEndpoint="/api/parent/enrollments/school-a/uploads" />,
    );

    pick(container, [new File(["%PDF-1.7"], "bescheid.pdf")]);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/parent/enrollments/school-a/uploads",
    );
  });

  it("rejects a file kind the field does not allow", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const { container } = render(<Harness />);

    pick(container, [new File(["x"], "foto.jpg", { type: "image/jpeg" })]);

    expect(
      screen.getByText(
        "Diese Datei hat die falsche Art. Erlaubt sind: PDF, PNG.",
      ),
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("offers only the allowed kinds to the file picker and the camera", () => {
    const { container } = render(<Harness />);
    const input =
      container.querySelector<HTMLInputElement>('input[type="file"]');
    expect(input).toHaveAttribute("accept", "application/pdf,image/png");
    expect(input).toHaveAttribute("multiple");
    expect(
      screen.getByText(
        "Erlaubt: PDF, PNG, höchstens 1 MB je Datei. Bis zu 5 Dateien. Auf dem Handy können Sie auch ein Foto machen.",
      ),
    ).toBeInTheDocument();
  });

  it("keeps a stored file until the parent removes it", () => {
    render(
      <Harness
        initial={[
          {
            upload_id: "55",
            filename: "alt.png",
            size_bytes: 4096,
            content_type: "image/png",
          },
        ]}
      />,
    );
    expect(screen.getByText("alt.png")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "alt.png entfernen" }));

    expect(changes.at(-1)).toEqual([]);
    expect(
      screen.getByRole("button", { name: "Datei auswählen" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("alt.png")).not.toBeInTheDocument();
  });

  it("uploads nothing in the preview", () => {
    render(<Harness previewMode />);
    expect(
      screen.getByRole("button", { name: "Datei auswählen" }),
    ).toBeDisabled();
    expect(
      screen.getByText("In der Vorschau können Sie keine Dateien hochladen."),
    ).toBeInTheDocument();
  });
});

describe("fileValueMissing", () => {
  it("counts only well-formed new or stored files", () => {
    expect(fileValueMissing(undefined)).toBe(true);
    expect(fileValueMissing([])).toBe(true);
    expect(fileValueMissing([{ filename: "ohne-token.pdf" }])).toBe(true);
    expect(
      fileValueMissing([
        { token: "t", filename: "a.pdf", size_bytes: 1, content_type: "x" },
      ]),
    ).toBe(false);
    expect(
      fileValueMissing([
        { upload_id: "9", filename: "a.pdf", size_bytes: 1, content_type: "x" },
      ]),
    ).toBe(false);
  });
});
