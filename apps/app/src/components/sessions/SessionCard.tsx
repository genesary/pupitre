import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { ApiError } from '@pupitre/core/lib/api-client';
import { bookSession, cancelBooking } from '@pupitre/core/lib/api';
import type { CourseSession } from '@pupitre/core/lib/api-types';
import { useSession } from '@pupitre/core/lib/session';

// An in-person session of a course, with its booking button (docs/specs/011-course-page-and-enrollment.md).

export function SessionCard({ slug, session, booked }: { slug: string; session: CourseSession; booked: boolean }) {
  const signedIn = Boolean(useSession());
  const queryClient = useQueryClient();
  const date = new Date(session.date);
  const past = date < new Date();
  const booking = useMutation({
    mutationFn: () => (booked ? cancelBooking(slug, session.id) : bookSession(slug, session.id)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['my', 'session-bookings'] }),
    onError: (error) =>
      toast.error(
        error instanceof ApiError && error.status === 409
          ? 'Cette session est complète.'
          : 'Une erreur est survenue, veuillez réessayer.',
      ),
  });
  return (
    <div className={`session-card ${booked ? 'session-card-booked' : 'session-card-default'}${past ? ' opacity-50' : ''}`}>
      <div className="session-card-inner">
        <div>
          <div className="session-title">{session.title}</div>
          <div className="session-meta-text">
            📅 {date.toLocaleDateString('fr-FR', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })} à{' '}
            {date.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}
          </div>
          {session.location && <div className="session-meta-text mt-0.5">📍 {session.location}</div>}
          {(session.capacity ?? 0) > 0 && <div className="session-capacity">🪑 {session.capacity} places</div>}
        </div>
        {!past && signedIn && (
          <button
            className={`btn ${booked ? 'btn-secondary' : 'btn-primary'} whitespace-nowrap shrink-0`}
            disabled={booking.isPending}
            onClick={() => booking.mutate()}
          >
            {booked ? '✓ Réservé — Annuler' : 'Réserver'}
          </button>
        )}
      </div>
    </div>
  );
}
