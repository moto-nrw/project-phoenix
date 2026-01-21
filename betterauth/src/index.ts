import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { toNodeHandler } from "better-auth/node";
import { auth } from "./auth.js";

const PORT = parseInt(process.env.PORT ?? "3001", 10);

// Create BetterAuth Node.js handler
const authHandler = toNodeHandler(auth);

/**
 * Simple HTTP server that handles BetterAuth API requests.
 *
 * All requests come through the Next.js proxy (server-to-server),
 * so no CORS handling is needed.
 *
 * BetterAuth provides a handler that processes all auth-related endpoints:
 * - POST /api/auth/sign-up/email
 * - POST /api/auth/sign-in/email
 * - POST /api/auth/sign-out
 * - GET /api/auth/session
 * - GET /api/auth/ok (health check)
 * - Organization endpoints (from plugin)
 * - etc.
 */
const server = createServer(
  async (req: IncomingMessage, res: ServerResponse) => {
    // Custom health check endpoint (outside BetterAuth)
    if (req.url === "/health" && req.method === "GET") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ status: "ok", service: "betterauth" }));
      return;
    }

    // Let BetterAuth handle the request
    // toNodeHandler handles all /api/auth/* routes
    try {
      await authHandler(req, res);
    } catch (error) {
      console.error("BetterAuth handler error:", error);
      if (!res.headersSent) {
        res.writeHead(500, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "Internal server error" }));
      }
    }
  }
);

server.listen(PORT, () => {
  console.log(`BetterAuth service listening on port ${PORT}`);
  console.log(`Health check: http://localhost:${PORT}/health`);
  console.log(`Auth endpoints: http://localhost:${PORT}/api/auth/*`);
});

// Graceful shutdown
process.on("SIGTERM", () => {
  console.log("SIGTERM received, shutting down gracefully...");
  server.close(() => {
    console.log("Server closed");
    process.exit(0);
  });
});

process.on("SIGINT", () => {
  console.log("SIGINT received, shutting down gracefully...");
  server.close(() => {
    console.log("Server closed");
    process.exit(0);
  });
});
