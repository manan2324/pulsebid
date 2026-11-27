import type { Metadata } from "next";
import { headers } from "next/headers";
import "@fontsource/inter/latin-400.css";
import "@fontsource/inter/latin-500.css";
import "@fontsource/inter/latin-600.css";
import "@fontsource/inter/latin-700.css";
import "@fontsource/jetbrains-mono/latin-400.css";
import "./globals.css";
import { Providers } from "@/components/providers";
import { copy } from "@/lib/copy/app";
export const metadata: Metadata = {
  title: { default: "PulseBid Console", template: "%s | PulseBid" },
  description: "PulseBid operator and exchange console",
  icons: { icon: "/icon.svg" },
};
export default async function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const nonce = (await headers()).get("x-nonce") ?? undefined;
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <a className="skip-link" href="#main-content">
          {copy.skip}
        </a>
        <Providers nonce={nonce}>{children}</Providers>
      </body>
    </html>
  );
}
