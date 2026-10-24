// Exercise Playwright's Node-side readiness check without an OS/hosts mapping.
// Chromium resolves the portal hosts independently as loopback addresses.
const dns = require("node:dns");

/** @param {string} hostname */
function unresolved(hostname) {
  return Object.assign(new Error("Network evidence: subdomain DNS disabled"), {
    code: "ENOTFOUND",
    syscall: "getaddrinfo",
    hostname,
  });
}

dns.lookup = new Proxy(dns.lookup, {
  apply(target, receiver, args) {
    const hostname = args[0];
    if (typeof hostname === "string" && hostname.endsWith(".localhost")) {
      const callback = args.at(-1);
      queueMicrotask(() => callback(unresolved(hostname)));
      return;
    }
    return Reflect.apply(target, receiver, args);
  },
});

dns.promises.lookup = new Proxy(dns.promises.lookup, {
  apply(target, receiver, args) {
    const hostname = args[0];
    if (typeof hostname === "string" && hostname.endsWith(".localhost")) {
      return Promise.reject(unresolved(hostname));
    }
    return Reflect.apply(target, receiver, args);
  },
});
