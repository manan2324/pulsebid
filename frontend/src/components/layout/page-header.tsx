import { copy } from "@/lib/copy/app";
export function PageHeader({
  title,
  description,
  phase,
}: {
  title: string;
  description: string;
  phase?: string;
}) {
  return (
    <div className="page-header">
      <div>
        <div className="eyebrow">{copy.console}</div>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {phase && (
        <span className="phase-chip">
          {copy.coming} {phase}
        </span>
      )}
    </div>
  );
}
