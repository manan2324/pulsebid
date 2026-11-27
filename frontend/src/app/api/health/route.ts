import { serviceHealth } from "@/lib/server/health";
import { requestId } from "@/lib/server/http";
export async function GET(request: Request) {
  const id = requestId(request);
  return Response.json(await serviceHealth(id), {
    headers: { "X-Request-Id": id, "Cache-Control": "no-store" },
  });
}
