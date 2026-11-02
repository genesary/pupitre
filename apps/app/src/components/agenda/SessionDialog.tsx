import { useMutation } from '@tanstack/react-query';
import { Dialog } from 'radix-ui';
import { useState, type FormEvent } from 'react';
import { toast } from 'sonner';

import { createSession, updateSession } from '@pupitre/core/lib/api';
import type { Course, CourseSession, SessionInput } from '@pupitre/core/lib/api-types';
import { toLocalInput } from '@/lib/calendar';

// « Nouvelle session » / « Modifier la session » dialog of the agenda
// (docs/specs/022-agenda-and-sessions.md), with the markup of agenda.astro's
// modal. A missing value is shown below its field and nothing is sent; the
// Astro page used alert().

export type SessionEdit = { courseSlug: string; session: CourseSession };

type Errors = Partial<Record<'course' | 'title' | 'date' | 'location' | 'capacity', string>>;

type SessionDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  courses: Course[];
  /** Absent: a new session. */
  editing: SessionEdit | null;
  onSaved: () => void;
};

export function SessionDialog({ open, onOpenChange, courses, editing, onSaved }: SessionDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="agenda-modal-overlay open">
          <Dialog.Content className="agenda-modal-box" aria-describedby={undefined}>
            {/* Remounted on each opening, so the form starts from the session edited. */}
            {open && <SessionForm courses={courses} editing={editing} onClose={() => onOpenChange(false)} onSaved={onSaved} />}
          </Dialog.Content>
        </Dialog.Overlay>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function SessionForm({
  courses,
  editing,
  onClose,
  onSaved,
}: {
  courses: Course[];
  editing: SessionEdit | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [course, setCourse] = useState(editing?.courseSlug ?? courses[0]?.slug ?? '');
  const [title, setTitle] = useState(editing?.session.title ?? '');
  const [date, setDate] = useState(editing ? toLocalInput(editing.session.date) : '');
  const [location, setLocation] = useState(editing?.session.location ?? '');
  const [capacity, setCapacity] = useState(String(editing?.session.capacity ?? 0));
  const [errors, setErrors] = useState<Errors>({});

  const save = useMutation({
    mutationFn: (body: SessionInput) =>
      editing ? updateSession(editing.courseSlug, editing.session.id, body) : createSession(course, body),
    onSuccess: () => {
      toast.success(editing ? 'Session modifiée' : 'Session créée');
      onSaved();
      onClose();
    },
    onError: (error) => toast.error(error.message || 'Erreur lors de la sauvegarde.'),
  });

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const capacityValue = Number(capacity);
    const next: Errors = {};
    if (!editing && !course) next.course = 'Choisissez un cours.';
    if (!title.trim()) next.title = 'Le titre est requis.';
    if (!date) next.date = 'La date et l’heure sont requises.';
    if (!location.trim()) next.location = 'Le lieu est requis.';
    if (capacity.trim() === '' || !Number.isInteger(capacityValue) || capacityValue < 0) {
      next.capacity = 'Un nombre entier, 0 pour illimité.';
    }
    setErrors(next);
    if (Object.keys(next).length > 0) return;
    save.mutate({ title: title.trim(), date: new Date(date).toISOString(), location: location.trim(), capacity: capacityValue });
  };

  const error = (field: keyof Errors) =>
    errors[field] && (
      <p id={`session-${field}-error`} className="text-bad text-[.8rem] mt-1">
        {errors[field]}
      </p>
    );
  const described = (field: keyof Errors) =>
    errors[field] ? { 'aria-invalid': true, 'aria-describedby': `session-${field}-error` } : {};

  return (
    <form onSubmit={submit} noValidate>
      <Dialog.Title className="modal-title">{editing ? 'Modifier la session' : 'Nouvelle session'}</Dialog.Title>
      <div className="form-group">
        <label className="form-label" htmlFor="session-course">
          Cours
        </label>
        <select
          className="form-input"
          id="session-course"
          value={course}
          disabled={Boolean(editing)}
          onChange={(event) => setCourse(event.target.value)}
          {...described('course')}
        >
          {courses.map((c) => (
            <option key={c.slug} value={c.slug}>
              {c.title}
            </option>
          ))}
        </select>
        {error('course')}
      </div>
      <div className="form-group">
        <label className="form-label" htmlFor="session-title">
          Titre
        </label>
        <input
          className="form-input"
          id="session-title"
          type="text"
          placeholder="ex: Session septembre — Groupe A"
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          {...described('title')}
        />
        {error('title')}
      </div>
      <div className="form-group">
        <label className="form-label" htmlFor="session-date">
          Date et heure
        </label>
        <input
          className="form-input"
          id="session-date"
          type="datetime-local"
          value={date}
          onChange={(event) => setDate(event.target.value)}
          {...described('date')}
        />
        {error('date')}
      </div>
      <div className="form-group">
        <label className="form-label" htmlFor="session-location">
          Lieu
        </label>
        <input
          className="form-input"
          id="session-location"
          type="text"
          placeholder="ex: Salle 3B, Paris"
          value={location}
          onChange={(event) => setLocation(event.target.value)}
          {...described('location')}
        />
        {error('location')}
      </div>
      <div className="form-group">
        <label className="form-label" htmlFor="session-capacity">
          Capacité
        </label>
        <input
          className="form-input"
          id="session-capacity"
          type="number"
          min={0}
          step={1}
          value={capacity}
          onChange={(event) => setCapacity(event.target.value)}
          {...described('capacity')}
        />
        {error('capacity')}
      </div>
      <div className="modal-actions">
        <button type="button" className="btn btn-secondary btn-sm" onClick={onClose}>
          Annuler
        </button>
        <button type="submit" className="btn btn-primary btn-sm" disabled={save.isPending}>
          {editing ? 'Enregistrer' : 'Créer'}
        </button>
      </div>
    </form>
  );
}
