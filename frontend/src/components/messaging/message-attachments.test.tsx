import { useState } from "react";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { uploadMessageAttachment } from "~/lib/message-attachments";
import type { MessageAttachment } from "~/lib/message-attachments";
import { AttachmentViewer } from "~/components/ui/attachment-viewer";
import { ChatBubble } from "./chat-bubble";
import { MessageComposer } from "./message-composer";

vi.mock("~/lib/message-attachments", async (original) => ({
  ...(await original<typeof import("~/lib/message-attachments")>()),
  uploadMessageAttachment: vi.fn(),
}));

const upload = vi.mocked(uploadMessageAttachment);

const PHOTO: MessageAttachment = {
  id: "11",
  filename: "ausflug.png",
  size_bytes: 2048,
  content_type: "image/png",
  width: 800,
  height: 600,
};
const LIST: MessageAttachment = {
  id: "12",
  filename: "bastelliste.pdf",
  size_bytes: 300 * 1024,
  content_type: "application/pdf",
};

function Composer({
  onSend,
}: Readonly<{ onSend: (files: readonly MessageAttachment[]) => void }>) {
  const [value, setValue] = useState("");
  return (
    <MessageComposer
      value={value}
      onChange={setValue}
      onSend={onSend}
      sending={false}
      attachments={{ uploadUrl: "/api/messages/attachments" }}
    />
  );
}

describe("Anhänge im Eingabefeld (#6685)", () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn(() => "blob:preview");
    URL.revokeObjectURL = vi.fn();
  });
  afterEach(() => vi.clearAllMocks());

  it("lädt eine gewählte Datei hoch und sendet sie ohne Text", async () => {
    const user = userEvent.setup();
    const onSend = vi.fn();
    upload.mockResolvedValue(PHOTO);
    render(<Composer onSend={onSend} />);

    const send = screen.getByRole("button", { name: "Senden" });
    expect(send).toBeDisabled();
    await user.upload(
      screen.getByLabelText("Foto oder Datei anhängen", { selector: "input" }),
      new File(["x"], "ausflug.png", { type: "image/png" }),
    );

    expect(upload).toHaveBeenCalledWith(
      "/api/messages/attachments",
      expect.objectContaining({ name: "ausflug.png" }),
    );
    await waitFor(() => expect(send).toBeEnabled());
    // Ein Foto zeigt sich selbst; sein Name ist der Bildname.
    expect(
      screen.getByRole("img", { name: "ausflug.png" }),
    ).toBeInTheDocument();

    await user.click(send);
    expect(onSend).toHaveBeenCalledWith([PHOTO]);
    // Nach dem Senden ist die Leiste leer.
    expect(
      screen.queryByRole("img", { name: "ausflug.png" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Nachricht" })).toBeVisible();
  });

  it("nimmt eine Datei an, die jemand in das Feld zieht", async () => {
    upload.mockResolvedValue(LIST);
    render(<Composer onSend={vi.fn()} />);
    const field = screen.getByRole("textbox", { name: "Nachricht" });
    const dataTransfer = {
      types: ["Files"],
      files: [new File(["x"], "bastelliste.pdf", { type: "application/pdf" })],
    };
    fireEvent.dragEnter(field, { dataTransfer });
    expect(screen.getByText("Dateien hier ablegen")).toBeInTheDocument();
    fireEvent.drop(field, { dataTransfer });
    expect(await screen.findByText("bastelliste.pdf")).toBeInTheDocument();
    expect(screen.queryByText("Dateien hier ablegen")).not.toBeInTheDocument();
  });

  it("nimmt einen Anhang wieder heraus", async () => {
    const user = userEvent.setup();
    upload.mockResolvedValue(LIST);
    render(<Composer onSend={vi.fn()} />);
    await user.upload(
      screen.getByLabelText("Foto oder Datei anhängen", { selector: "input" }),
      new File(["x"], "bastelliste.pdf", { type: "application/pdf" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Anhang entfernen: bastelliste.pdf",
      }),
    );
    expect(screen.queryByText("bastelliste.pdf")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Senden" })).toBeDisabled();
  });

  it("öffnet einen Anhang über seine Kachel, ohne ihn zu entfernen", async () => {
    const user = userEvent.setup();
    upload.mockResolvedValue(PHOTO);
    const onOpenAttachment = vi.fn();
    render(
      <MessageComposer
        value=""
        onChange={vi.fn()}
        onSend={vi.fn()}
        sending={false}
        attachments={{
          uploadUrl: "/api/messages/attachments",
          onOpenAttachment,
        }}
      />,
    );
    await user.upload(
      screen.getByLabelText("Foto oder Datei anhängen", { selector: "input" }),
      new File(["x"], "ausflug.png", { type: "image/png" }),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Senden" })).toBeEnabled(),
    );

    await user.click(
      screen.getByRole("button", { name: "Anhang öffnen: ausflug.png" }),
    );
    expect(onOpenAttachment).toHaveBeenCalledTimes(1);
    expect(onOpenAttachment).toHaveBeenCalledWith({
      file: expect.objectContaining({ name: "ausflug.png" }),
      previewUrl: "blob:preview",
      attachment: PHOTO,
    });
    // Das Kreuz bleibt ein eigener Knopf: Öffnen nimmt nichts heraus.
    expect(
      screen.getByRole("button", { name: "Anhang entfernen: ausflug.png" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "ausflug.png" }),
    ).toBeInTheDocument();
  });

  it("zeigt einen gewählten Anhang in der Vorschau, bevor er gesendet ist", async () => {
    const user = userEvent.setup();
    upload.mockResolvedValue(LIST);
    render(<Composer onSend={vi.fn()} />);
    await user.upload(
      screen.getByLabelText("Foto oder Datei anhängen", { selector: "input" }),
      new File(["x"], "bastelliste.pdf", { type: "application/pdf" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Anhang öffnen: bastelliste.pdf",
      }),
    );
    const viewer = screen.getByRole("dialog", { name: "bastelliste.pdf" });
    await user.click(
      within(viewer).getAllByRole("button", { name: "Schließen" })[0]!,
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    // Die Adresse aus dem Gerät ist wieder freigegeben, der Anhang bleibt.
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:preview");
    expect(screen.getByText("bastelliste.pdf")).toBeInTheDocument();
  });

  it("sagt, warum eine Datei nicht angehängt wird", async () => {
    const user = userEvent.setup({ applyAccept: false });
    render(<Composer onSend={vi.fn()} />);
    await user.upload(
      screen.getByLabelText("Foto oder Datei anhängen", { selector: "input" }),
      new File(["x"], "film.mp4", { type: "video/mp4" }),
    );
    expect(
      await screen.findByText(/„film\.mp4“ kann nicht angehängt werden/),
    ).toBeInTheDocument();
    expect(upload).not.toHaveBeenCalled();
  });
});

describe("Anhänge in der Sprechblase (#6685)", () => {
  const url = (id: string) => `/files/${id}`;

  it("zeigt Fotos als Bilder und Dateien als Zeile zum Herunterladen", () => {
    render(
      <ChatBubble
        body="Fotos von heute"
        own={false}
        senderName="OGS"
        createdAt="2026-08-16T06:50:00Z"
        attachments={[PHOTO, LIST]}
        attachmentUrl={url}
      />,
    );
    expect(screen.getByText("Fotos von heute")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "ausflug.png" })).toHaveAttribute(
      "src",
      "/files/11?inline=1",
    );
    const download = screen.getByRole("link", {
      name: "Herunterladen: bastelliste.pdf",
    });
    expect(download).toHaveAttribute("href", "/files/12");
    expect(screen.getByText("PDF · 300 KB")).toBeInTheDocument();
    // Der Name öffnet die Ansicht, der Pfeil speichert sofort.
    expect(
      screen.getByRole("button", { name: "Datei öffnen: bastelliste.pdf" }),
    ).toHaveTextContent("bastelliste.pdf");
  });

  it("zeigt ein PDF in der Ansicht und bietet es zum Speichern an", async () => {
    const user = userEvent.setup();
    URL.createObjectURL = vi.fn(() => "blob:pdf");
    URL.revokeObjectURL = vi.fn();
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(new Blob(["%PDF-1.7"]), {
        status: 200,
        headers: { "Content-Type": "application/pdf" },
      }),
    );
    render(
      <ChatBubble
        body=""
        own={false}
        senderName="OGS"
        createdAt="2026-08-16T06:50:00Z"
        attachments={[PHOTO, LIST]}
        attachmentUrl={url}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "Datei öffnen: bastelliste.pdf" }),
    );
    const viewer = screen.getByRole("dialog", { name: "bastelliste.pdf" });
    // Fotos und Dateien einer Nachricht blättern in einer Ansicht.
    expect(within(viewer).getByText("2 von 2")).toBeInTheDocument();
    expect(await within(viewer).findByTitle("bastelliste.pdf")).toHaveAttribute(
      "src",
      "blob:pdf#view=FitH&navpanes=0",
    );
    expect(fetchSpy).toHaveBeenCalledWith(
      "/files/12?inline=1",
      expect.objectContaining({ method: "GET" }),
    );
    expect(
      within(viewer).getByRole("link", { name: "In neuem Tab öffnen" }),
    ).toHaveAttribute("href", "blob:pdf");
    expect(
      within(viewer).getByRole("link", { name: "Herunterladen" }),
    ).toHaveAttribute("href", "/files/12");
    expect(
      within(viewer).getByRole("link", {
        name: "Herunterladen: bastelliste.pdf",
      }),
    ).toHaveAttribute("download", "bastelliste.pdf");
    fetchSpy.mockRestore();
  });

  it("bietet eine Word-Datei in der Ansicht zum Herunterladen an", async () => {
    const user = userEvent.setup();
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    render(
      <ChatBubble
        body=""
        own={false}
        senderName="OGS"
        createdAt="2026-08-16T06:50:00Z"
        attachments={[
          {
            id: "14",
            filename: "elternbrief.docx",
            size_bytes: 24 * 1024,
            content_type:
              "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
          },
        ]}
        attachmentUrl={url}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "Datei öffnen: elternbrief.docx" }),
    );
    const viewer = screen.getByRole("dialog", { name: "elternbrief.docx" });
    expect(
      within(viewer).getByText(
        "Diese Datei lässt sich hier nicht anzeigen. Laden Sie sie herunter, um sie zu öffnen.",
      ),
    ).toBeInTheDocument();
    expect(within(viewer).getByText("Word · 24 KB")).toBeInTheDocument();
    const save = within(viewer).getByRole("link", { name: "Herunterladen" });
    expect(save).toHaveAttribute("href", "/files/14");
    expect(save).toHaveAttribute("download", "elternbrief.docx");
    expect(within(viewer).queryByTitle("elternbrief.docx")).toBeNull();
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });

  it("zeigt eine Datei vom Gerät und speichert sie ohne eigene Adresse", () => {
    render(
      <AttachmentViewer
        items={[
          {
            src: "blob:entwurf",
            name: "entwurf.pdf",
            contentType: "application/pdf",
            sizeBytes: 1024,
          },
        ]}
        index={0}
        onIndexChange={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    const viewer = screen.getByRole("dialog", { name: "entwurf.pdf" });
    expect(within(viewer).getByTitle("entwurf.pdf")).toHaveAttribute(
      "src",
      "blob:entwurf#view=FitH&navpanes=0",
    );
    expect(
      within(viewer).getByRole("link", { name: "Herunterladen: entwurf.pdf" }),
    ).toHaveAttribute("href", "blob:entwurf");
  });

  it("öffnet ein Foto groß und schließt es wieder", async () => {
    const user = userEvent.setup();
    render(
      <ChatBubble
        body=""
        own
        senderName="Ich"
        createdAt="2026-08-16T06:50:00Z"
        attachments={[PHOTO, { ...PHOTO, id: "13", filename: "drachen.png" }]}
        attachmentUrl={url}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "Bild öffnen: ausflug.png" }),
    );
    const viewer = screen.getByRole("dialog", { name: "ausflug.png" });
    expect(within(viewer).getByText("1 von 2")).toBeInTheDocument();
    expect(
      within(viewer).getByRole("link", { name: "Herunterladen: ausflug.png" }),
    ).toHaveAttribute("href", "/files/11");
    await user.click(
      within(viewer).getByRole("button", { name: "Nächster Anhang" }),
    );
    expect(
      screen.getByRole("dialog", { name: "drachen.png" }),
    ).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    // Die Fotos stehen weiter in der Unterhaltung.
    expect(
      screen.getByRole("button", { name: "Bild öffnen: ausflug.png" }),
    ).toBeInTheDocument();
  });
});

describe("Fehlgeschlagenes Senden (#6685)", () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn(() => "blob:preview");
    URL.revokeObjectURL = vi.fn();
  });

  it("behält die Anhänge, wenn die Nachricht nicht ankommt", async () => {
    const user = userEvent.setup();
    upload.mockResolvedValue(PHOTO);
    const onSend = vi.fn().mockResolvedValue(false);
    render(<Composer onSend={onSend} />);
    await user.upload(
      screen.getByLabelText("Foto oder Datei anhängen", { selector: "input" }),
      new File(["x"], "ausflug.png", { type: "image/png" }),
    );
    const send = screen.getByRole("button", { name: "Senden" });
    await waitFor(() => expect(send).toBeEnabled());
    await user.click(send);

    expect(onSend).toHaveBeenCalledWith([PHOTO]);
    // Die Datei steht wieder im Feld und lässt sich erneut senden.
    expect(
      await screen.findByRole("img", { name: "ausflug.png" }),
    ).toBeInTheDocument();
    await waitFor(() => expect(send).toBeEnabled());
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();
  });
});
