import { getEnv } from "@/lib/server/env";
import { proxy } from "@/lib/server/proxy";
import { apiError, requestId } from "@/lib/server/http";
async function handle(
  request: Request,
  context: { params: Promise<{ action: string }> },
) {
  const { action } = await context.params;
  const valid =
    action === "bid"
      ? request.method === "POST"
      : ["impression", "click"].includes(action) && request.method === "GET";
  const url = new URL(request.url);
  if (
    !valid ||
    (action !== "bid" &&
      [...url.searchParams.keys()].some((key) => key !== "t"))
  )
    return apiError(
      404,
      "not_found",
      "Unknown gateway route or method",
      requestId(request),
    );
  const path =
    action === "bid" ? "/v1/bid" : `/v1/events/${action}${url.search}`;
  return proxy(request, {
    base: getEnv().GATEWAY_URL,
    path,
    service: "Gateway",
    timeout: 3000,
    gateway: true,
    click: action === "click",
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
