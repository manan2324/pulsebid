"use client";
import { useEffect, useRef, useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
export function OnVisible({
  children,
  title,
}: {
  children: React.ReactNode;
  title: string;
}) {
  const container = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    if (!container.current) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "100px" },
    );
    observer.observe(container.current);
    return () => observer.disconnect();
  }, []);
  return (
    <div ref={container} className="deferred-panel">
      {visible ? (
        children
      ) : (
        <section className="panel lab-card">
          <h2>{title}</h2>
          <Skeleton className="h-64 w-full" />
        </section>
      )}
    </div>
  );
}
