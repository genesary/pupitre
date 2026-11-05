import { Dialog } from 'radix-ui';
import type { CSSProperties, ReactNode } from 'react';

// A modal with the markup of the Astro pages (an overlay holding a card), on
// Radix Dialog for focus trapping, Escape and outside click.

type ModalProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  /** Class of the overlay; pupitre's overlays are hidden until shown as flex. */
  overlayClassName?: string;
  contentClassName: string;
  contentStyle?: CSSProperties;
  titleClassName?: string;
  /** Shown next to the title, e.g. a close button. */
  header?: (title: ReactNode) => ReactNode;
  children: ReactNode;
};

export function Modal({
  open,
  onOpenChange,
  title,
  overlayClassName = 'fixed inset-0 z-[200] items-center justify-center bg-black/40',
  contentClassName,
  contentStyle,
  titleClassName,
  header,
  children,
}: ModalProps) {
  const heading = <Dialog.Title className={titleClassName}>{title}</Dialog.Title>;
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className={overlayClassName} style={{ display: 'flex' }}>
          <Dialog.Content className={contentClassName} style={contentStyle} aria-describedby={undefined}>
            {header ? header(heading) : heading}
            {children}
          </Dialog.Content>
        </Dialog.Overlay>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/** The « ✕ » button of a modal header. */
export function ModalClose({ className }: { className?: string }) {
  return (
    <Dialog.Close className={className} aria-label="Fermer">
      ✕
    </Dialog.Close>
  );
}
