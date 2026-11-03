// A same-origin document that every portal host serves with 200: the proxy
// matcher skips this static file. Portal hosts answer /api/* outside their own
// APIs (such as /api/health on the school and caterer hosts) with an empty 404,
// and Chrome turns that into net::ERR_HTTP_RESPONSE_CODE_FAILURE (#5612).
export const ORIGIN_PAGE = "/favicon.ico";
