"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { useReportCatalog } from "@/hooks/use-reports";
import { copy } from "@/lib/copy/app";
import { polishCopy as c } from "@/lib/copy/polish";
import {
  Command,
  CommandInput,
  CommandList,
  CommandItem,
  CommandGroup,
  CommandEmpty,
} from "@/components/ui/command";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
export function CommandPalette({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const router = useRouter(),
    { resolvedTheme, setTheme } = useTheme();
  const campaigns = useReportCatalog(open);
  const [term, setTerm] = useState("");
  const [help, setHelp] = useState(false);
  const go = (href: string) => {
    onClose();
    router.push(href);
  };
  const rows =
    campaigns.data
      ?.filter((row) => row.name.toLowerCase().includes(term.toLowerCase()))
      .slice(0, 50) ?? [];
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!value) onClose();
      }}
    >
      <DialogContent aria-describedby="palette-description">
        <DialogTitle>{c.palette}</DialogTitle>
        <DialogDescription id="palette-description">
          {c.searchHelp}
        </DialogDescription>
        <Command>
          <CommandInput
            aria-label={c.search}
            placeholder={c.search}
            value={term}
            onValueChange={setTerm}
          />
          <CommandList>
            <CommandEmpty>{c.empty}</CommandEmpty>
            <CommandGroup heading={c.navigation}>
              {copy.nav.map((item) => (
                <CommandItem
                  key={item.href}
                  value={`page ${item.label}`}
                  onSelect={() => go(item.href)}
                >
                  {item.label}
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandGroup heading={c.actions}>
              <CommandItem onSelect={() => go("/campaigns/new")}>
                {c.create}
              </CommandItem>
              <CommandItem
                onSelect={() => {
                  setTheme(resolvedTheme === "dark" ? "light" : "dark");
                  onClose();
                }}
              >
                {c.theme}
              </CommandItem>
              <CommandItem onSelect={() => setHelp((value) => !value)}>
                {c.help}
              </CommandItem>
            </CommandGroup>
            <CommandGroup heading={c.campaigns}>
              {rows.map((row) => (
                <CommandItem
                  value={`${row.name} ${row.id}`}
                  key={row.id}
                  onSelect={() => go(`/campaigns/${row.id}`)}
                >
                  {row.name}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
        {campaigns.isError && <p role="status">{c.searchError}</p>}
        {campaigns.isPending && <p role="status">{c.loading}</p>}
        <small>{c.found}</small>
        {help && <p>{c.shortcutHelp}</p>}
      </DialogContent>
    </Dialog>
  );
}
