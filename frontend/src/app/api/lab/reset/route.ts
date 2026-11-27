import { restoreSchema } from "@/lib/lab";
import { getEnv } from "@/lib/server/env";
import { apiError, requestId, prepareMutation } from "@/lib/server/http";
export async function POST(request: Request) {
  const id = requestId(request);
  const prepared = await prepareMutation(request, id);
  if (prepared.response) return prepared.response;
  let body: unknown;
  try {
    body = JSON.parse(new TextDecoder().decode(prepared.body));
  } catch {
    return apiError(400, "invalid_request", "Invalid restore payload", id);
  }
  const parsed = restoreSchema.safeParse(body);
  const env = getEnv();
  if (
    !parsed.success ||
    parsed.data.entries.some((entry) => entry.index >= env.BIDDER_COUNT) ||
    new Set(parsed.data.entries.map((entry) => entry.index)).size !==
      parsed.data.entries.length
  )
    return apiError(400, "invalid_request", "Invalid bidder restoration", id);
  const results = await Promise.all(
    parsed.data.entries.map(async ({ index, config }) => {
      try {
        const response = await fetch(
          `http://${env.BIDDER_HOST}:${env.BIDDER_BASE_PORT + index}/admin/config`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json", "X-Request-Id": id },
            body: JSON.stringify(config),
            signal: AbortSignal.timeout(2000),
            redirect: "manual",
            cache: "no-store",
          },
        );
        await response.arrayBuffer();
        return { index, restored: response.ok };
      } catch {
        return { index, restored: false };
      }
    }),
  );
  const failed = results.filter((item) => !item.restored);
  return Response.json(
    {
      results,
      ...(failed.length
        ? {
            error: {
              code: "unavailable",
              message: "Some bidders could not be restored",
              request_id: id,
              details: failed.map((item) => ({
                field: `bidders.${item.index}`,
                issue: "Restoration failed",
              })),
            },
          }
        : {}),
    },
    {
      status: results.every((item) => item.restored) ? 200 : 503,
      headers: { "X-Request-Id": id, "Cache-Control": "no-store" },
    },
  );
}
