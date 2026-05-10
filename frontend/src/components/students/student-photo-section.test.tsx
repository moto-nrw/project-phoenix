import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { StudentPhotoSection } from "./student-photo-section";
import type { Student } from "~/lib/api";

// PhotoSection is a *controlled component* — picking a file just calls
// onPickPhoto(blob); removing calls onMarkRemoved(). The actual upload
// happens in the parent's submit handler when the user clicks Speichern.
// Tests reflect that contract: no API mocks, only callback assertions.

const compressAvatar = vi.fn(async (f: Blob) => f);
vi.mock("~/lib/image-utils", () => ({
  compressAvatar: (f: Blob) => compressAvatar(f),
}));

// Defensive: jsdom doesn't ship URL.createObjectURL — stub it so the
// avatar preview blob URL doesn't throw during render.
beforeEach(() => {
  if (!("createObjectURL" in URL)) {
    Object.defineProperty(URL, "createObjectURL", {
      value: vi.fn(() => "blob:mock"),
      configurable: true,
    });
  }
  if (!("revokeObjectURL" in URL)) {
    Object.defineProperty(URL, "revokeObjectURL", {
      value: vi.fn(),
      configurable: true,
    });
  }
  compressAvatar.mockClear();
});

function makeStudent(overrides: Partial<Student> = {}): Student {
  return {
    id: "42",
    name: "Tristan Heitger",
    first_name: "Tristan",
    second_name: "Heitger",
    school_class: "1a",
    current_location: "Anwesend",
    ...overrides,
  };
}

describe("StudentPhotoSection (controlled)", () => {
  it("hides upload + delete buttons when consent is unchecked", () => {
    render(
      <StudentPhotoSection
        student={makeStudent()}
        consentGiven={false}
        onConsentChange={() => {}}
        pendingPhotoBlob={null}
        pendingPhotoRemoved={false}
        onPickPhoto={() => {}}
        onMarkRemoved={() => {}}
        onCancelRemove={() => {}}
      />,
    );

    expect(
      screen.queryByRole("button", { name: /Foto auswählen/i }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: /Foto entfernen/i }),
    ).toBeNull();
  });

  it("shows the pick button immediately when consent is checked, no save needed", () => {
    render(
      <StudentPhotoSection
        student={makeStudent()}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={null}
        pendingPhotoRemoved={false}
        onPickPhoto={() => {}}
        onMarkRemoved={() => {}}
        onCancelRemove={() => {}}
      />,
    );

    expect(
      screen.getByRole("button", { name: /Foto auswählen/i }),
    ).toBeInTheDocument();
    // No photo on the row → no remove button yet
    expect(
      screen.queryByRole("button", { name: /Foto entfernen/i }),
    ).toBeNull();
  });

  it("shows 'Foto ersetzen' + 'Foto entfernen' when student already has a photo", () => {
    render(
      <StudentPhotoSection
        student={makeStudent({
          photo_url: "/api/students/42/photo/abc.jpg",
          photo_consent_given_at: "2026-05-06T10:00:00Z",
        })}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={null}
        pendingPhotoRemoved={false}
        onPickPhoto={() => {}}
        onMarkRemoved={() => {}}
        onCancelRemove={() => {}}
      />,
    );

    expect(
      screen.getByRole("button", { name: /Foto ersetzen/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Foto entfernen/i }),
    ).toBeInTheDocument();
  });

  it("compresses the picked file and calls onPickPhoto with the blob — does NOT upload", async () => {
    const onPickPhoto = vi.fn();

    const { container } = render(
      <StudentPhotoSection
        student={makeStudent()}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={null}
        pendingPhotoRemoved={false}
        onPickPhoto={onPickPhoto}
        onMarkRemoved={() => {}}
        onCancelRemove={() => {}}
      />,
    );

    const fileInput = container.querySelector(
      "input[type=file]",
    ) as HTMLInputElement;
    const file = new File(["x"], "p.jpg", { type: "image/jpeg" });
    Object.defineProperty(fileInput, "files", { value: [file] });
    fireEvent.change(fileInput);

    await waitFor(() => expect(onPickPhoto).toHaveBeenCalledTimes(1));
    expect(compressAvatar).toHaveBeenCalledWith(file);
    // The first arg is the compressed Blob — our mock returns the input
    // unchanged, so we just assert it's a Blob.
    const [blob] = onPickPhoto.mock.calls[0] as [Blob];
    expect(blob).toBeInstanceOf(Blob);
  });

  it("renders the 'wird beim Speichern hochgeladen' hint while a pick is pending", () => {
    const blob = new Blob(["x"], { type: "image/jpeg" });
    render(
      <StudentPhotoSection
        student={makeStudent()}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={blob}
        pendingPhotoRemoved={false}
        onPickPhoto={() => {}}
        onMarkRemoved={() => {}}
        onCancelRemove={() => {}}
      />,
    );

    expect(
      screen.getByText(/wird beim Speichern hochgeladen/i),
    ).toBeInTheDocument();
    // The remove button now reads 'Auswahl verwerfen' — clicking it should
    // un-pick (call onPickPhoto(null)), not mark for deletion.
    expect(
      screen.getByRole("button", { name: /Auswahl verwerfen/i }),
    ).toBeInTheDocument();
  });

  it("clicking 'Auswahl verwerfen' clears the pending pick (onPickPhoto(null))", () => {
    const onPickPhoto = vi.fn();
    const onMarkRemoved = vi.fn();
    const blob = new Blob(["x"], { type: "image/jpeg" });

    render(
      <StudentPhotoSection
        student={makeStudent()}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={blob}
        pendingPhotoRemoved={false}
        onPickPhoto={onPickPhoto}
        onMarkRemoved={onMarkRemoved}
        onCancelRemove={() => {}}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Auswahl verwerfen/i }));
    expect(onPickPhoto).toHaveBeenCalledWith(null);
    expect(onMarkRemoved).not.toHaveBeenCalled();
  });

  it("clicking 'Foto entfernen' on a server photo calls onMarkRemoved (no immediate API)", () => {
    const onPickPhoto = vi.fn();
    const onMarkRemoved = vi.fn();

    render(
      <StudentPhotoSection
        student={makeStudent({
          photo_url: "/api/students/42/photo/abc.jpg",
          photo_consent_given_at: "2026-05-06T10:00:00Z",
        })}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={null}
        pendingPhotoRemoved={false}
        onPickPhoto={onPickPhoto}
        onMarkRemoved={onMarkRemoved}
        onCancelRemove={() => {}}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Foto entfernen/i }));
    expect(onMarkRemoved).toHaveBeenCalledTimes(1);
    expect(onPickPhoto).not.toHaveBeenCalled();
  });

  // Regression for [P3]: after a user clicks "Foto entfernen" on a server
  // photo, the secondary action button must stay visible so they can
  // reverse the pending deletion before clicking Speichern. The original
  // implementation hid it (hasAnyPhoto went false once pendingPhotoRemoved
  // was true), leaving the user stuck unless they uploaded a replacement
  // or abandoned the form.
  it("keeps the secondary action visible after marking removal so the user can undo", () => {
    const onCancelRemove = vi.fn();
    const onMarkRemoved = vi.fn();
    const onPickPhoto = vi.fn();

    render(
      <StudentPhotoSection
        student={makeStudent({
          photo_url: "/api/students/42/photo/abc.jpg",
          photo_consent_given_at: "2026-05-06T10:00:00Z",
        })}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={null}
        // Simulate the state after the user clicked "Foto entfernen".
        pendingPhotoRemoved={true}
        onPickPhoto={onPickPhoto}
        onMarkRemoved={onMarkRemoved}
        onCancelRemove={onCancelRemove}
      />,
    );

    // Button reads "Entfernung rückgängig" and is still rendered.
    const undoButton = screen.getByRole("button", {
      name: /Entfernung rückgängig/i,
    });
    expect(undoButton).toBeInTheDocument();

    // The pending-removal hint is shown so the user knows what's queued.
    expect(
      screen.getByText(/Foto wird beim Speichern entfernt/i),
    ).toBeInTheDocument();

    // Clicking the button reverses the removal, does NOT re-mark or
    // touch onPickPhoto.
    fireEvent.click(undoButton);
    expect(onCancelRemove).toHaveBeenCalledTimes(1);
    expect(onMarkRemoved).not.toHaveBeenCalled();
    expect(onPickPhoto).not.toHaveBeenCalled();
  });

  it("forwards consent toggle clicks to onConsentChange", () => {
    const onConsentChange = vi.fn();

    render(
      <StudentPhotoSection
        student={makeStudent()}
        consentGiven={false}
        onConsentChange={onConsentChange}
        pendingPhotoBlob={null}
        pendingPhotoRemoved={false}
        onPickPhoto={() => {}}
        onMarkRemoved={() => {}}
        onCancelRemove={() => {}}
      />,
    );

    fireEvent.click(screen.getByLabelText(/Einwilligung der Eltern/i));
    expect(onConsentChange).toHaveBeenCalledWith(true);
  });

  // Compress-failure path: when image-utils.compressAvatar throws (e.g. user
  // picked a corrupt or oversize file the canvas can't decode), the
  // component must surface a German error string and NOT call onPickPhoto
  // — so the parent's submit handler doesn't try to upload garbage.
  it("renders an error and skips onPickPhoto when compressAvatar throws", async () => {
    compressAvatar.mockImplementationOnce(async () => {
      throw new Error("Bild zu groß");
    });
    const onPickPhoto = vi.fn();
    const { container } = render(
      <StudentPhotoSection
        student={makeStudent()}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={null}
        pendingPhotoRemoved={false}
        onPickPhoto={onPickPhoto}
        onMarkRemoved={() => {}}
        onCancelRemove={() => {}}
      />,
    );

    const fileInput = container.querySelector(
      "input[type=file]",
    ) as HTMLInputElement;
    const file = new File(["x"], "p.jpg", { type: "image/jpeg" });
    Object.defineProperty(fileInput, "files", { value: [file] });
    fireEvent.change(fileInput);

    await waitFor(() =>
      expect(screen.getByText(/Bild zu groß/i)).toBeInTheDocument(),
    );
    expect(onPickPhoto).not.toHaveBeenCalled();
  });

  // The primary "Foto auswählen" button delegates to the hidden file input
  // via fileInputRef. Clicking it must call .click() on the input element —
  // covers the onClick handler arrow function.
  it("clicking 'Foto auswählen' triggers the hidden file input", () => {
    const { container } = render(
      <StudentPhotoSection
        student={makeStudent()}
        consentGiven={true}
        onConsentChange={() => {}}
        pendingPhotoBlob={null}
        pendingPhotoRemoved={false}
        onPickPhoto={() => {}}
        onMarkRemoved={() => {}}
        onCancelRemove={() => {}}
      />,
    );

    const fileInput = container.querySelector(
      "input[type=file]",
    ) as HTMLInputElement;
    const clickSpy = vi.spyOn(fileInput, "click");

    fireEvent.click(screen.getByRole("button", { name: /Foto auswählen/i }));
    expect(clickSpy).toHaveBeenCalledTimes(1);
  });
});
