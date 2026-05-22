const apiBaseUrl = requiredEnv("API_BASE_URL").replace(/\/$/, "");
const webJwt = requiredEnv("WEB_JWT");
const connections = numberEnv("SSE_CONNECTIONS", 300);
const holdMs = numberEnv("SSE_HOLD_SECONDS", 900) * 1000;
const rampMs = numberEnv("SSE_RAMP_SECONDS", 60) * 1000;

const controllers = [];
let opened = 0;
let failed = 0;

process.on("SIGINT", () => {
  closeAll();
  process.exit(130);
});

console.log(`Opening ${connections} SSE connections over ${rampMs / 1000}s`);

for (let i = 0; i < connections; i++) {
  const delay = Math.floor((rampMs / Math.max(connections, 1)) * i);
  setTimeout(() => {
    void openConnection(i);
  }, delay);
}

const report = setInterval(() => {
  console.log(JSON.stringify({
    opened,
    failed,
    target: connections,
    active_controllers: controllers.length,
  }));
}, 5000);

setTimeout(() => {
  clearInterval(report);
  closeAll();
  console.log(`Closed SSE connections: opened=${opened} failed=${failed}`);
  process.exit(failed > 0 ? 1 : 0);
}, holdMs + rampMs + 1000);

async function openConnection(index) {
  const controller = new AbortController();
  controllers.push(controller);

  try {
    const response = await fetch(`${apiBaseUrl}/api/sse/events?loadtest=${index}`, {
      headers: {
        Authorization: `Bearer ${webJwt}`,
        Accept: "text/event-stream",
      },
      signal: controller.signal,
    });

    if (!response.ok || !response.body) {
      failed++;
      console.error(`SSE ${index} failed to open: ${response.status}`);
      return;
    }

    opened++;
    const reader = response.body.getReader();
    while (!controller.signal.aborted) {
      const { done } = await reader.read();
      if (done) break;
    }
  } catch (error) {
    if (!controller.signal.aborted) {
      failed++;
      console.error(`SSE ${index} error: ${error instanceof Error ? error.message : String(error)}`);
    }
  }
}

function closeAll() {
  for (const controller of controllers) {
    controller.abort();
  }
}

function requiredEnv(name) {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

function numberEnv(name, fallback) {
  const value = process.env[name];
  if (!value) return fallback;
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    throw new Error(`${name} must be a non-negative number`);
  }
  return parsed;
}
