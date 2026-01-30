import { describe, it, expect, vi, beforeEach } from "vitest";
import { compressAvatar } from "./image-utils";
import imageCompression from "browser-image-compression";

vi.mock("browser-image-compression", () => ({
  default: vi.fn(),
}));

describe("compressAvatar", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
  });

  it("successfully compresses image and returns File with .jpg extension", async () => {
    const mockBlob = new Blob(["compressed"], { type: "image/jpeg" });
    vi.mocked(imageCompression).mockResolvedValue(mockBlob);

    const originalFile = new File(["original"], "avatar.png", {
      type: "image/png",
    });

    const result = await compressAvatar(originalFile);

    expect(imageCompression).toHaveBeenCalledWith(originalFile, {
      maxSizeMB: 0.3,
      maxWidthOrHeight: 512,
      useWebWorker: true,
      fileType: "image/jpeg",
    });

    expect(result).toBeInstanceOf(File);
    expect(result.name).toBe("avatar.jpg");
    expect(result.type).toBe("image/jpeg");
  });

  it("handles files with multiple dots in filename", async () => {
    const mockBlob = new Blob(["compressed"], { type: "image/jpeg" });
    vi.mocked(imageCompression).mockResolvedValue(mockBlob);

    const originalFile = new File(["original"], "my.avatar.image.png", {
      type: "image/png",
    });

    const result = await compressAvatar(originalFile);

    expect(result.name).toBe("my.avatar.image.jpg");
  });

  it("handles files without extension", async () => {
    const mockBlob = new Blob(["compressed"], { type: "image/jpeg" });
    vi.mocked(imageCompression).mockResolvedValue(mockBlob);

    const originalFile = new File(["original"], "avatar", {
      type: "image/png",
    });

    const result = await compressAvatar(originalFile);

    expect(result.name).toBe("avatar.jpg");
  });

  it("returns original file when compression fails", async () => {
    const error = new Error("Compression failed");
    vi.mocked(imageCompression).mockRejectedValue(error);

    const originalFile = new File(["original"], "avatar.png", {
      type: "image/png",
    });

    const result = await compressAvatar(originalFile);

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "Avatar compression failed, using original:",
      error,
    );
    expect(result).toBe(originalFile);
  });

  it("creates File with correct lastModified timestamp", async () => {
    const mockBlob = new Blob(["compressed"], { type: "image/jpeg" });
    vi.mocked(imageCompression).mockResolvedValue(mockBlob);

    const originalFile = new File(["original"], "avatar.png", {
      type: "image/png",
    });

    const beforeTimestamp = Date.now();
    const result = await compressAvatar(originalFile);
    const afterTimestamp = Date.now();

    expect(result.lastModified).toBeGreaterThanOrEqual(beforeTimestamp);
    expect(result.lastModified).toBeLessThanOrEqual(afterTimestamp);
  });

  it("preserves filename without extension correctly", async () => {
    const mockBlob = new Blob(["compressed"], { type: "image/jpeg" });
    vi.mocked(imageCompression).mockResolvedValue(mockBlob);

    const testCases = [
      { input: "photo.jpg", expected: "photo.jpg" },
      { input: "photo.jpeg", expected: "photo.jpg" },
      { input: "photo.PNG", expected: "photo.jpg" },
      { input: "photo.webp", expected: "photo.jpg" },
    ];

    for (const { input, expected } of testCases) {
      const file = new File(["data"], input, { type: "image/png" });
      const result = await compressAvatar(file);
      expect(result.name).toBe(expected);
    }
  });

  it("verifies compression options are correct", async () => {
    const mockBlob = new Blob(["compressed"], { type: "image/jpeg" });
    vi.mocked(imageCompression).mockResolvedValue(mockBlob);

    const originalFile = new File(["original"], "avatar.png", {
      type: "image/png",
    });

    await compressAvatar(originalFile);

    const calledOptions = vi.mocked(imageCompression).mock.calls[0]?.[1];

    expect(calledOptions).toEqual({
      maxSizeMB: 0.3,
      maxWidthOrHeight: 512,
      useWebWorker: true,
      fileType: "image/jpeg",
    });
  });
});
