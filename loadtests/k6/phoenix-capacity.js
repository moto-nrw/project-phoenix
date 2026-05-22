import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";

const apiBaseUrl = requiredEnv("API_BASE_URL").replace(/\/$/, "");
const deviceApiKey = requiredEnv("DEVICE_API_KEY");
const staffPin = requiredEnv("STAFF_PIN");
const webJwt = optionalEnv("WEB_JWT");
const rfids = requiredEnv("RFIDS").split(",").map((value) => value.trim()).filter(Boolean);
const roomIds = optionalEnv("ROOM_IDS")
  .split(",")
  .map((value) => value.trim())
  .filter(Boolean);

const scanRequests = new Counter("phoenix_scan_requests");
const scanFailures = new Rate("phoenix_scan_failures");
const dashboardFailures = new Rate("phoenix_dashboard_failures");
const scanLatency = new Trend("phoenix_scan_latency_ms");

export const options = {
  scenarios: {
    device_scan_burst: {
      executor: "ramping-arrival-rate",
      exec: "deviceScan",
      startRate: numberEnv("SCAN_START_RATE", 5),
      timeUnit: "1s",
      preAllocatedVUs: numberEnv("SCAN_PREALLOCATED_VUS", 80),
      maxVUs: numberEnv("SCAN_MAX_VUS", 300),
      stages: [
        { duration: envDuration("SCAN_RAMP_DURATION", "5m"), target: numberEnv("SCAN_PEAK_RATE", 100) },
        { duration: envDuration("SCAN_HOLD_DURATION", "10m"), target: numberEnv("SCAN_PEAK_RATE", 100) },
        { duration: envDuration("SCAN_RAMP_DOWN_DURATION", "2m"), target: 0 },
      ],
    },
    web_dashboard_reads: {
      executor: "constant-vus",
      exec: "dashboardRead",
      vus: numberEnv("WEB_VUS", 0),
      duration: envDuration("WEB_DURATION", "17m"),
      startTime: "0s",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{scenario:device_scan_burst}": ["p(95)<500", "p(99)<1000"],
    "http_req_duration{scenario:web_dashboard_reads}": ["p(95)<300", "p(99)<1000"],
    phoenix_scan_failures: ["rate<0.005"],
    phoenix_dashboard_failures: ["rate<0.01"],
  },
};

export function deviceScan() {
  const rfid = pick(rfids);
  const payload = {
    student_rfid: rfid,
    action: "checkin",
  };
  const roomID = pick(roomIds);
  if (roomID) {
    payload.room_id = Number(roomID);
  }

  const response = http.post(`${apiBaseUrl}/api/iot/checkin`, JSON.stringify(payload), {
    headers: deviceHeaders(),
    tags: { endpoint: "iot_checkin" },
    timeout: "10s",
  });

  scanRequests.add(1);
  scanLatency.add(response.timings.duration);
  const ok = check(response, {
    "scan status is non-5xx": (res) => res.status < 500,
    "scan status is expected": (res) => [200, 400, 404, 409, 422].includes(res.status),
  });
  scanFailures.add(!ok);
  sleep(Math.random() * 0.2);
}

export function dashboardRead() {
  if (!webJwt) {
    sleep(1);
    return;
  }

  const responses = http.batch([
    ["GET", `${apiBaseUrl}/api/me`, null, { headers: webHeaders(), tags: { endpoint: "me" }, timeout: "10s" }],
    ["GET", `${apiBaseUrl}/api/me/groups/active`, null, { headers: webHeaders(), tags: { endpoint: "active_groups" }, timeout: "10s" }],
    ["GET", `${apiBaseUrl}/api/active/analytics/dashboard`, null, { headers: webHeaders(), tags: { endpoint: "dashboard_analytics" }, timeout: "10s" }],
  ]);

  const ok = responses.every((res) => res.status >= 200 && res.status < 500);
  dashboardFailures.add(!ok);
  sleep(numberEnv("WEB_THINK_TIME_SECONDS", 5));
}

function deviceHeaders() {
  return {
    Authorization: `Bearer ${deviceApiKey}`,
    "X-Staff-PIN": staffPin,
    "Content-Type": "application/json",
  };
}

function webHeaders() {
  return {
    Authorization: `Bearer ${webJwt}`,
    Accept: "application/json",
  };
}

function pick(values) {
  if (values.length === 0) return "";
  return values[Math.floor(Math.random() * values.length)];
}

function requiredEnv(name) {
  const value = __ENV[name];
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

function optionalEnv(name) {
  return __ENV[name] || "";
}

function numberEnv(name, fallback) {
  const value = __ENV[name];
  if (!value) return fallback;
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    throw new Error(`${name} must be a non-negative number`);
  }
  return parsed;
}

function envDuration(name, fallback) {
  return __ENV[name] || fallback;
}
