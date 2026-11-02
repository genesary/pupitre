import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router';
import { toast } from 'sonner';

import { ApiError } from '@pupitre/core/lib/api-client';
import { bookSession, cancelBooking, deleteSession, listCourses, mySessionBookings } from '@pupitre/core/lib/api';
import type { Course, CourseSession } from '@pupitre/core/lib/api-types';
import { dayKey, monthGrid, MONTHS_SHORT } from '@/lib/calendar';
import { useCurrentUser } from '@pupitre/core/lib/session';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@pupitre/ui/components/ui/alert-dialog';
import { SessionDialog, type SessionEdit } from '@/components/agenda/SessionDialog';
import { SiteLink } from '@pupitre/core/components/SiteLink';

// /agenda (docs/specs/022-agenda-and-sessions.md), markup of pupitre's agenda.astro:
// a monthly calendar of the in-person sessions and a side panel to book them.
// Admins and managers also create, edit and delete sessions.

type Item = { course: Course; session: CourseSession; date: Date };

const bookingKey = (slug: string, sessionId: string) => `${slug}|${sessionId}`;

function SessionItem({
  item,
  booked,
  trainer,
  onEdit,
  onDelete,
}: {
  item: Item;
  booked: boolean;
  trainer: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const queryClient = useQueryClient();
  const { course, session, date } = item;
  const past = date < new Date();
  const booking = useMutation({
    mutationFn: () => (booked ? cancelBooking(course.slug, session.id) : bookSession(course.slug, session.id)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['my', 'session-bookings'] }),
    onError: (error) =>
      toast.error(
        error instanceof ApiError && error.status === 409
          ? 'Cette session est complète.'
          : 'Une erreur est survenue, veuillez réessayer.',
      ),
  });

  return (
    <div className={`ev-card${booked ? ' booked' : ''}`}>
      <div className="ev-course">
        <Link to={`/courses/${course.slug}`} className="text-inherit no-underline">
          {course.title}
        </Link>
      </div>
      <div className="ev-title">{session.title}</div>
      <div className="ev-meta">
        📅 {date.toLocaleDateString('fr-FR', { day: 'numeric', month: 'long' })} à{' '}
        {date.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}
        {session.location && (
          <>
            <br />📍 {session.location}
          </>
        )}
        {(session.capacity ?? 0) > 0 && (
          <>
            <br />🪑 {session.capacity} places
          </>
        )}
      </div>
      {booked && <div className="ev-booked-badge">✓ Inscrit(e)</div>}
      {!past && (
        <button
          className={`btn ${booked ? 'btn-secondary' : 'btn-primary'} btn-sm w-full`}
          disabled={booking.isPending}
          onClick={() => booking.mutate()}
        >
          {booked ? '✓ Réservé — Annuler' : 'Réserver'}
        </button>
      )}
      {trainer && (
        <>
          {/* The attendance page is in the admin app. */}
          <SiteLink
            to={`/admin/courses/sessions/${course.slug}/${session.id}`}
            className="btn btn-secondary btn-sm w-full text-center no-underline block mt-1.5"
          >
            👥 Présence
          </SiteLink>
          <div className="ev-trainer-actions">
            <button className="btn btn-secondary btn-sm flex-1" onClick={onEdit}>
              Modifier
            </button>
            <button className="btn btn-secondary btn-sm flex-1 text-[#dc2626]" onClick={onDelete}>
              Supprimer
            </button>
          </div>
        </>
      )}
    </div>
  );
}

function MonthPicker({ year, month, onPick }: { year: number; month: number; onPick: (year: number, month: number) => void }) {
  const [open, setOpen] = useState(false);
  const [pickerYear, setPickerYear] = useState(year);
  const wrap = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => {
      if (!wrap.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('click', close);
    return () => document.removeEventListener('click', close);
  }, [open]);

  return (
    <div className="mp-wrap" ref={wrap}>
      <button
        className="cal-month-btn"
        title="Choisir mois / année"
        aria-expanded={open}
        onClick={() => {
          setPickerYear(year);
          setOpen((value) => !value);
        }}
      >
        {new Date(year, month, 1).toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })}
      </button>
      <div className={`mp-popup${open ? ' open' : ''}`}>
        <div className="mp-year-row">
          <button className="mp-year-btn" aria-label="Année précédente" onClick={() => setPickerYear((y) => y - 1)}>
            ‹
          </button>
          <span className="mp-year-lbl">{pickerYear}</span>
          <button className="mp-year-btn" aria-label="Année suivante" onClick={() => setPickerYear((y) => y + 1)}>
            ›
          </button>
        </div>
        <div className="mp-months">
          {MONTHS_SHORT.map((label, index) => (
            <button
              key={label}
              className={`mp-month-btn${pickerYear === year && index === month ? ' active' : ''}`}
              onClick={() => {
                setOpen(false);
                onPick(pickerYear, index);
              }}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

export function Agenda() {
  const user = useCurrentUser();
  const trainer = user?.role === 'admin' || user?.role === 'manager';
  const queryClient = useQueryClient();
  const courses = useQuery({ queryKey: ['courses'], queryFn: listCourses });
  const bookings = useQuery({ queryKey: ['my', 'session-bookings'], queryFn: mySessionBookings });

  const now = new Date();
  const today = dayKey(now);
  // null until the user navigates: the agenda then opens on the month of the
  // next session, like the Astro page.
  const [view, setView] = useState<{ year: number; month: number } | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [dialog, setDialog] = useState<{ open: boolean; editing: SessionEdit | null }>({ open: false, editing: null });
  const [toDelete, setToDelete] = useState<Item | null>(null);

  const inPerson = useMemo(() => (courses.data?.courses ?? []).filter((c) => c.inPerson), [courses.data]);
  const items = useMemo(
    () =>
      inPerson
        .flatMap((course) => (course.sessions ?? []).map((session) => ({ course, session, date: new Date(session.date) })))
        .sort((a, b) => a.date.getTime() - b.date.getTime()),
    [inPerson],
  );
  const booked = useMemo(
    () => new Set((bookings.data?.bookings ?? []).map((b) => bookingKey(b.courseSlug, b.sessionId))),
    [bookings.data],
  );
  const byDay = useMemo(() => {
    const map = new Map<string, Item[]>();
    for (const item of items) {
      const key = dayKey(item.date);
      map.set(key, [...(map.get(key) ?? []), item]);
    }
    return map;
  }, [items]);

  const nextSession = items.find((item) => item.date >= now)?.date ?? now;
  const year = view?.year ?? nextSession.getFullYear();
  const month = view?.month ?? nextSession.getMonth();

  const goTo = (nextYear: number, nextMonth: number) => {
    const date = new Date(nextYear, nextMonth, 1);
    setView({ year: date.getFullYear(), month: date.getMonth() });
    setSelected(null);
  };

  const remove = useMutation({
    mutationFn: (item: Item) => deleteSession(item.course.slug, item.session.id),
    onSuccess: () => {
      toast.success('Session supprimée');
      void queryClient.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (error) => toast.error(error.message || 'Erreur lors de la suppression.'),
  });

  const panelItems = selected ? (byDay.get(selected) ?? []) : items.filter((item) => item.date >= now).slice(0, 10);
  const panelTitle = selected
    ? new Date(`${selected}T12:00:00`)
        .toLocaleDateString('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })
        .toUpperCase()
    : 'Sessions à venir';

  return (
    <div className="agenda-wrap">
      <div className="cal-area">
        <div className="cal-topbar">
          <span className="text-[1.3rem]">📅</span>
          <span className="cal-topbar-title" data-tour="page-title">Agenda</span>
          <button
            className="cal-today-btn"
            onClick={() => {
              setView({ year: now.getFullYear(), month: now.getMonth() });
              setSelected(today);
            }}
          >
            Aujourd'hui
          </button>
          <div className="cal-nav-group">
            <button className="cal-nav-btn" aria-label="Mois précédent" onClick={() => goTo(year, month - 1)}>
              ‹
            </button>
            <MonthPicker year={year} month={month} onPick={goTo} />
            <button className="cal-nav-btn" aria-label="Mois suivant" onClick={() => goTo(year, month + 1)}>
              ›
            </button>
          </div>
          {courses.isPending && <div className="text-[.8rem] text-ink-soft">Chargement…</div>}
          {courses.isError && (
            <button className="text-[.8rem] text-bad underline" onClick={() => courses.refetch()}>
              Erreur de chargement. Réessayer
            </button>
          )}
          {trainer && (
            <button
              className="btn-new-session"
              style={{ display: 'block' }}
              onClick={() => setDialog({ open: true, editing: null })}
            >
              + Nouvelle session
            </button>
          )}
        </div>
        <div className="cal-dow-row">
          {['Lun', 'Mar', 'Mer', 'Jeu', 'Ven', 'Sam', 'Dim'].map((day) => (
            <div key={day} className="cal-dow-cell">
              {day}
            </div>
          ))}
        </div>
        <div className="cal-body">
          {monthGrid(year, month).map((cell) => {
            const sessions = byDay.get(cell.key) ?? [];
            const clickable = sessions.length > 0 || cell.key === today;
            const className = [
              'cal-cell',
              cell.otherMonth && 'other-month',
              cell.key === today && 'today',
              cell.key === selected && 'selected',
              sessions.length > 0 && 'has-event',
            ]
              .filter(Boolean)
              .join(' ');
            return (
              <div
                key={cell.key}
                className={className}
                role={clickable ? 'button' : undefined}
                tabIndex={clickable ? 0 : undefined}
                aria-label={clickable ? `${cell.day} : ${sessions.length} session(s)` : undefined}
                onClick={clickable ? () => setSelected(cell.key) : undefined}
                onKeyDown={
                  clickable
                    ? (event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          setSelected(cell.key);
                        }
                      }
                    : undefined
                }
              >
                <div className="cal-day-num">{cell.day}</div>
                {sessions.slice(0, 3).map((item) => (
                  <div
                    key={bookingKey(item.course.slug, item.session.id)}
                    className={`cal-event-pill${booked.has(bookingKey(item.course.slug, item.session.id)) ? ' booked' : ''}`}
                    title={`${item.course.title} — ${item.session.title}`}
                  >
                    {item.session.title}
                  </div>
                ))}
                {sessions.length > 3 && (
                  <div className="text-[.7rem] text-ink-soft pl-1">+{sessions.length - 3} autres</div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      <div className="side-panel">
        <div className="side-header">{panelTitle}</div>
        <div className="side-body">
          {panelItems.length === 0 ? (
            <div className="no-sel">{selected ? 'Aucune session ce jour.' : 'Aucune session à venir.'}</div>
          ) : (
            panelItems.map((item) => (
              <SessionItem
                key={bookingKey(item.course.slug, item.session.id)}
                item={item}
                booked={booked.has(bookingKey(item.course.slug, item.session.id))}
                trainer={trainer}
                onEdit={() => setDialog({ open: true, editing: { courseSlug: item.course.slug, session: item.session } })}
                onDelete={() => setToDelete(item)}
              />
            ))
          )}
        </div>
      </div>

      {trainer && (
        <>
          <SessionDialog
            open={dialog.open}
            onOpenChange={(open) => setDialog((current) => ({ ...current, open }))}
            courses={inPerson}
            editing={dialog.editing}
            onSaved={() => void queryClient.invalidateQueries({ queryKey: ['courses'] })}
          />
          <AlertDialog open={toDelete !== null} onOpenChange={(open) => !open && setToDelete(null)}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Supprimer cette session ?</AlertDialogTitle>
                <AlertDialogDescription>
                  {toDelete ? `« ${toDelete.session.title} » (${toDelete.course.title}) sera supprimée.` : ''}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Annuler</AlertDialogCancel>
                <AlertDialogAction
                  variant="destructive"
                  onClick={() => {
                    if (toDelete) remove.mutate(toDelete);
                  }}
                >
                  Supprimer
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </>
      )}
    </div>
  );
}
