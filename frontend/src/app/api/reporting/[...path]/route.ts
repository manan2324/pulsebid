import { getEnv } from "@/lib/server/env";
import { proxy } from "@/lib/server/proxy";
import { apiError, requestId } from "@/lib/server/http";
export async function GET(
  request: Request,
  context: { params: Promise<{ path: string[] }> },
) {
  const { path } = await context.params;
  const joined = path.join("/");
  const url = new URL(request.url);
  if (
    !/^reports\/campaigns(?:\/[a-zA-Z0-9][a-zA-Z0-9_-]{0,127})?$/.test(
      joined,
    ) ||
    [...url.searchParams.keys()].some(
      (key) => !["from", "to", "granularity", "campaign_id"].includes(key),
    )
  )
    return apiError(
      404,
      "not_found",
      "Unknown reporting route or parameter",
      requestId(request),
    );
  return proxy(request, {
    base: getEnv().REPORTING_API_URL,
    path: `/v1/${joined}${url.search}`,
    service: "Reporting service",
  });
}
