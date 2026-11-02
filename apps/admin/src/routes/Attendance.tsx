import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { toast } from 'sonner';

import { getCourse, markPresence, sessionParticipants } from '@pupitre/core/lib/api';
import type { SessionParticipant } from '@pupitre/core/lib/api-types';
import { useCurrentUser } from '@pupitre/core/lib/session';
import { LoadError } from '@pupitre/core/components/PageStates';
import { SiteLink } from '@pupitre/core/components/SiteLink';

// /admin/courses/sessions/:slug/:sessionId (docs/specs/022-agenda-and-sessions.md),
// markup of pupitre's admin/courses/sessions/[slug]/[sessionId].astro. An admin
// reads /api/admin/…, a manager /api/manager/… (spec 002).

type Participants = { participants: SessionParticipant[] | null };

const longDate = (iso: string) =>
  new Date(iso).toLocaleDateString('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' });

export function Attendance() {
  const { slug = '', sessionId = '' } = useParams();
  const role = useCurrentUser()?.role === 'admin' ? 'admin' : 'manager';
  const queryClient = useQueryClient();
  const key = ['session-participants', role, slug, sessionId];

  const course = useQuery({ queryKey: ['course', slug], queryFn: () => getCourse(slug) });
  const participants = useQuery({ queryKey: key, queryFn: () => sessionParticipants(role, slug, sessionId) });

  // Optimistic: the box changes at once and goes back if the call fails.
  const presence = useMutation({
    mutationFn: ({ userId, present }: { userId: string; present: boolean }) =>
      markPresence(role, slug, sessionId, userId, present),
    onMutate: async ({ userId, present }) => {
      await queryClient.cancelQueries({ queryKey: key });
      const previous = queryClient.getQueryData<Participants>(key);
      queryClient.setQueryData<Participants>(key, (data) => ({
        participants: (data?.participants ?? []).map((p) => (p.userId === userId ? { ...p, present } : p)),
      }));
      return { previous };
    },
    onError: (_error, _variables, context) => {
      queryClient.setQueryData(key, context?.previous);
      toast.error('Erreur lors de la mise à jour de la présence.');
    },
  });

  const session = course.data?.sessions?.find((s) => s.id === sessionId);
  const list = participants.data?.participants ?? [];
  const present = list.filter((p) => p.present).length;

  return (
    <div className="presence-outer">
      <div className="page-inner">
        <SiteLink to="/agenda" className="presence-back">
          ← Retour à l'agenda
        </SiteLink>
        <div className="presence-title">
          {participants.isPending ? 'Chargement…' : session?.title ? `Session — ${session.title}` : `Session ${sessionId}`}
        </div>
        <div className="presence-sub">
          {course.data?.title
            ? `${course.data.title}${session?.date ? ` · ${longDate(session.date)}` : ''}`
            : participants.isPending
              ? ''
              : slug}
        </div>

        {participants.isError ? (
          <LoadError error={participants.error} onRetry={() => participants.refetch()} />
        ) : participants.isSuccess ? (
          <>
            <div className="presence-stats">
              <div className="stat-chip">
                <span>{list.length}</span>Inscrits
              </div>
              <div className="stat-chip green">
                <span>{present}</span>Présents
              </div>
              <div className="stat-chip">
                <span>{list.length - present}</span>Absents
              </div>
            </div>
            {list.length === 0 ? (
              <div className="no-participants">Aucun participant pour cette session.</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="presence-table">
                  <thead>
                    <tr>
                      <th>Participant</th>
                      <th>Email</th>
                      <th>Inscrit le</th>
                      <th className="text-center">Présent</th>
                    </tr>
                  </thead>
                  <tbody>
                    {list.map((p) => (
                      <tr key={p.userId}>
                        <td>
                          <strong>{p.username}</strong>
                        </td>
                        <td className="text-ink-soft">{p.email}</td>
                        <td className="text-ink-soft">{longDate(p.bookedAt)}</td>
                        <td className="text-center">
                          <input
                            type="checkbox"
                            className="presence-cb"
                            aria-label={`${p.username} présent`}
                            checked={p.present}
                            onChange={(event) => presence.mutate({ userId: p.userId, present: event.target.checked })}
                          />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        ) : null}
      </div>
    </div>
  );
}
