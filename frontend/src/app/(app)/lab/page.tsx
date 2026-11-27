import { LabPage } from "@/components/lab/lab-page";
import { getEnv } from "@/lib/server/env";
export default function Page() {
  const env = getEnv();
  return <LabPage count={env.BIDDER_COUNT} basePort={env.BIDDER_BASE_PORT} />;
}
