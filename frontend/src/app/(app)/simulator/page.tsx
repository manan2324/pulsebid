import { SimulatorPage } from "@/components/simulator/simulator-page";
import { getEnv } from "@/lib/server/env";
export default function Page() {
  const env = getEnv();
  return (
    <SimulatorPage
      gatewayUrl={env.GATEWAY_PUBLIC_URL}
      testMode={env.TEST_MODE_UI === "true"}
    />
  );
}
