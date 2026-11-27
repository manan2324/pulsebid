import Link from "next/link";
import { copy } from "@/lib/copy/app";
export default function NotFound() {
  return (
    <main id="main-content" className="standalone-state">
      <span className="eyebrow">404</span>
      <h1>{copy.notFound}</h1>
      <p>{copy.notFoundDescription}</p>
      <Link href="/">{copy.backHome}</Link>
    </main>
  );
}
