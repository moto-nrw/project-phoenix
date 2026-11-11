import { useState } from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { uploadMessageAttachment } from "~/lib/message-attachments";
import type { MessageAttachment } from "~/lib/message-attachments";
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
    expect(screen.getByText("ausflug.png")).toBeInTheDocument();

    await user.click(send);
    expect(onSend).toHaveBeenCalledWith([PHOTO]);
    // Nach dem Senden ist die Leiste leer.
    expect(screen.queryByText("ausflug.png")).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Nachricht" })).toBeVisible();
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
    await user.click(
      within(viewer).getByRole("button", { name: "Nächstes Bild" }),
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
    expect(await screen.findByText("ausflug.png")).toBeInTheDocument();
    await waitFor(() => expect(send).toBeEnabled());
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();
  });
});
