// Defence in depth for the Next process, including accidental proxy traffic.
// No request URL, query, headers or body is included in an error.
const http = require("node:http");
const https = require("node:https");

/** @param {string | URL | Request} input */
function check(input) {
  const url = new URL(
    typeof input === "string" || input instanceof URL ? input : input.url,
  );
  if (
    url.protocol !== "http:" ||
    (!["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) &&
      !url.hostname.endsWith(".localhost"))
  ) {
    throw new Error("Network evidence: non-local server request blocked");
  }
}

const originalFetch = globalThis.fetch;
/** @type {typeof globalThis.fetch} */
const localFetch = (input, options) => {
  check(input);
  return originalFetch(input, options);
};
globalThis.fetch = localFetch;
for (const transport of [http, https]) {
  for (const method of /** @type {const} */ (["request", "get"])) {
    const original = transport[method];
    transport[method] = new Proxy(original, {
      apply(target, receiver, args) {
        const input = args[0];
        if (typeof input === "string" || input instanceof URL) check(input);
        else
          check(
            `${input.protocol ?? (transport === https ? "https:" : "http:")}//${input.hostname ?? input.host ?? "localhost"}`,
          );
        return Reflect.apply(target, receiver, args);
      },
    });
  }
}
