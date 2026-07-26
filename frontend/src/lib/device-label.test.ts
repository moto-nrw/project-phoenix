import { describe, expect, it } from "vitest";
import { formatDeviceLabelFromUserAgent } from "./device-label";

describe("formatDeviceLabelFromUserAgent", () => {
  it("formats Chrome on macOS", () => {
    expect(
      formatDeviceLabelFromUserAgent(
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
      ),
    ).toBe("Chrome auf macOS");
  });

  it("formats Safari on iPhone", () => {
    expect(
      formatDeviceLabelFromUserAgent(
        "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
      ),
    ).toBe("Safari auf iPhone");
  });

  it("formats Firefox on Windows", () => {
    expect(
      formatDeviceLabelFromUserAgent(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0",
      ),
    ).toBe("Firefox auf Windows");
  });

  it("falls back when the user agent is empty or unknown", () => {
    expect(formatDeviceLabelFromUserAgent("")).toBe("Unbekanntes Gerät");
    expect(formatDeviceLabelFromUserAgent("CustomAgent/1.0")).toBe(
      "Unbekanntes Gerät",
    );
  });
});
