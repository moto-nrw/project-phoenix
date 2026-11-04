// One source for the synthetic suites' ports: the Playwright config, the
// fixture server and the specs all read the same environment variables, so a
// run on other ports stays consistent (#5209). Playwright passes its own
// environment to the server and the workers.
const suites = {
  network: {
    frontend: { name: "NETWORK_E2E_PORT", fallback: 13843 },
    api: { name: "NETWORK_E2E_API_PORT", fallback: 18443 },
  },
  tenantSwitch: {
    frontend: { name: "TENANT_SWITCH_PORT", fallback: 13845 },
    api: { name: "TENANT_SWITCH_API_PORT", fallback: 18445 },
  },
};

/** @param {{ name: string, fallback: number }} variable */
function readPort({ name, fallback }) {
  const raw = process.env[name];
  if (raw === undefined || raw === "") return fallback;
  const port = Number(raw);
  if (!/^\d+$/.test(raw) || port < 1024 || port > 65535) {
    throw new Error(`${name} must be a port between 1024 and 65535`);
  }
  return port;
}

/**
 * @param {keyof typeof suites} suite
 * @returns {{ frontend: number, api: number, names: { frontend: string, api: string } }}
 */
export function networkPorts(suite) {
  const { frontend, api } = suites[suite];
  const ports = {
    frontend: readPort(frontend),
    api: readPort(api),
    names: { frontend: frontend.name, api: api.name },
  };
  if (ports.frontend === ports.api) {
    throw new Error(`${frontend.name} and ${api.name} must differ`);
  }
  return ports;
}
