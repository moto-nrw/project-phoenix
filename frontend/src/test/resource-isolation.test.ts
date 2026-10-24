import { afterEach, expect, it } from "vitest";
import type { Window } from "happy-dom";

const browser = (window as unknown as Window).happyDOM;
const originalInterceptor = browser.settings.fetch.interceptor;

afterEach(() => {
  document.body.replaceChildren();
  document.head
    .querySelectorAll("[data-network-proof]")
    .forEach((node) => node.remove());
  browser.settings.fetch.interceptor = originalInterceptor;
});

it("does not load iframe pages or stylesheets from rendered component markup", async () => {
  const attempts: string[] = [];
  // Even on the pre-fix configuration the countercheck never sends traffic.
  browser.settings.fetch.interceptor = {
    beforeAsyncRequest: async ({ request, window: browserWindow }) => {
      attempts.push(new URL(request.url).origin);
      return new browserWindow.Response("", { status: 200 });
    },
  };
  const iframe = document.createElement("iframe");
  iframe.src = "https://marketing.invalid/";
  document.body.append(iframe);
  const stylesheet = document.createElement("link");
  stylesheet.rel = "stylesheet";
  stylesheet.href = "https://marketing.invalid/style.css";
  stylesheet.dataset.networkProof = "true";
  document.head.append(stylesheet);
  await browser.waitUntilComplete();
  // Anker: both resource-bearing elements reached the document.
  expect(iframe).toBeInTheDocument();
  expect(stylesheet).toBeInTheDocument();
  expect(attempts).toEqual([]);
});

it("does not fetch documents opened in a new window", async () => {
  const attempts: string[] = [];
  browser.settings.fetch.interceptor = {
    beforeAsyncRequest: async ({ request, window: browserWindow }) => {
      attempts.push(new URL(request.url).origin);
      return new browserWindow.Response(null, { status: 204 });
    },
  };
  const link = document.createElement("a");
  link.href = "http://localhost:3000/network-isolation-proof";
  link.target = "_blank";
  link.textContent = "Open local fixture";
  document.body.append(link);
  // Anker: a real, connected link exercises Happy DOM's native navigation.
  expect(link).toBeInTheDocument();
  link.click();
  await browser.waitUntilComplete();
  expect(attempts).toEqual([]);
});
