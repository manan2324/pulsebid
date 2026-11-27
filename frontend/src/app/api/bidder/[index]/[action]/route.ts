import { getEnv } from "@/lib/server/env";
import { proxy, bidderIndex } from "@/lib/server/proxy";
import { apiError, requestId } from "@/lib/server/http";

async function handle(
  request: Request,
  context: { params: Promise<{ index: string; action: string }> },
) {
  const { index, action } = await context.params;
  const env = getEnv();
  if (
    !bidderIndex(index, env.BIDDER_COUNT) ||
    !(action === "config"
      ? ["GET", "POST"].includes(request.method)
      : action === "health" && request.method === "GET")
  )
    return apiError(
      404,
      "not_found",
      "Unknown bidder route or method",
      requestId(request),
    );
  return proxy(request, {
    base: `http://${env.BIDDER_HOST}:${env.BIDDER_BASE_PORT + Number(index)}`,
    path: action === "health" ? "/healthz" : "/admin/config",
    service: `Bidder ${Number(index) + 1}`,
    timeout: action === "health" ? 2000 : 10000,
  });
}

export {
  handle as GET,
  handle as POST,
  handle as PUT,
  handle as PATCH,
  handle as DELETE,
  handle as HEAD,
  handle as OPTIONS,
};
