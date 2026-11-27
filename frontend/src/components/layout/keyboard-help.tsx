"use client";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { polishCopy as c } from "@/lib/copy/polish";
export function KeyboardHelp({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!value) onClose();
      }}
    >
      <DialogContent>
        <DialogTitle>{c.help}</DialogTitle>
        <DialogDescription>{c.shortcutHelp}</DialogDescription>
      </DialogContent>
    </Dialog>
  );
}
