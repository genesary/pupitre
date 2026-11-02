import { toast } from 'sonner';

import { Modal, ModalClose } from '@pupitre/core/components/Modal';

// Read-only « Définition » dialog with « Copier » (docs/specs/042, 043, 045),
// markup of the Astro admin pages' modal-box.

export function DefinitionModal({ title, text, onClose }: { title: string; text: string; onClose: () => void }) {
  return (
    <Modal
      open
      onOpenChange={(open) => !open && onClose()}
      title={title}
      overlayClassName="modal-overlay open"
      contentClassName="modal-box"
      header={(heading) => (
        <div className="modal-head">
          <h3>{heading}</h3>
          <ModalClose className="btn btn-ghost btn-sm" />
        </div>
      )}
    >
      <div className="modal-body">
        <pre className="yaml">{text}</pre>
      </div>
      <div className="modal-foot">
        <button
          className="btn btn-secondary btn-sm"
          onClick={() => void navigator.clipboard.writeText(text).then(() => toast.success('Copié'))}
        >
          Copier
        </button>
        <button className="btn btn-secondary btn-sm" onClick={onClose}>
          Fermer
        </button>
      </div>
    </Modal>
  );
}
