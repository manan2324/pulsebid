import { BiddersPage } from "@/components/catalog/bidders-page";
import { getEnv } from "@/lib/server/env";

export default function Page() {
  const env = getEnv();
  return (
    <BiddersPage basePort={env.BIDDER_BASE_PORT} count={env.BIDDER_COUNT} />
  );
}
