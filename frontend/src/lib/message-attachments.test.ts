import { describe, expect, it } from "vitest";

import {
  attachmentIdArgs,
  checkAttachmentFiles,
  guardianMessageAttachments,
  MAX_MESSAGE_ATTACHMENT_MB,
} from "./message-attachments";

function file(name: string, bytes = 10): File {
  const blob = new File(["x"], name);
  Object.defineProperty(blob, "size", { value: bytes });
  return blob;
}

describe("checkAttachmentFiles", () => {
  it("nimmt Fotos und Dokumente an", () => {
    const { accepted, refusals } = checkAttachmentFiles(0, [
      file("foto.JPG"),
      file("liste.pdf"),
      file("plan.xlsx"),
    ]);
    expect(accepted.map((f) => f.name)).toEqual([
      "foto.JPG",
      "liste.pdf",
      "plan.xlsx",
    ]);
    expect(refusals).toEqual([]);
  });

  it("nennt die Datei, die zu groß ist oder nicht erlaubt", () => {
    const { accepted, refusals } = checkAttachmentFiles(0, [
      file("video.mp4"),
      file("riesig.png", MAX_MESSAGE_ATTACHMENT_MB * 1024 * 1024 + 1),
      file("gut.png"),
    ]);
    expect(accepted.map((f) => f.name)).toEqual(["gut.png"]);
    expect(refusals).toEqual([
      { kind: "type", filename: "video.mp4" },
      { kind: "too_large", filename: "riesig.png" },
    ]);
  });

  it("hält die Höchstzahl je Nachricht und meldet sie einmal", () => {
    const { accepted, refusals } = checkAttachmentFiles(3, [
      file("a.png"),
      file("b.png"),
      file("c.png"),
      file("d.png"),
    ]);
    expect(accepted.map((f) => f.name)).toEqual(["a.png", "b.png"]);
    expect(refusals).toEqual([{ kind: "too_many" }]);
  });
});

describe("Routen und Sende-Argument", () => {
  it("hängt die Anhänge der Eltern an das Kind", () => {
    const routes = guardianMessageAttachments("42");
    expect(routes.uploadUrl).toBe(
      "/api/parent/me/messages/children/42/attachments",
    );
    expect(routes.downloadUrl("7")).toBe(
      "/api/parent/me/messages/children/42/attachments/7/download",
    );
  });

  it("lässt das Argument ohne Anhänge weg", () => {
    expect(attachmentIdArgs([])).toEqual([]);
    expect(
      attachmentIdArgs([
        { id: "9", filename: "a.png", size_bytes: 1, content_type: "image/png" },
      ]),
    ).toEqual([["9"]]);
  });
});
