# 010 - Catalogue

Features: F040 (catalogue API), F041 (catalogue page), F042 (skill filter), F043 ("Reprendre"
hero), F044 (course card).

## Route

`/courses`, public. The page is enriched when the user is signed in.

## Data

| Call | When | Use |
| --- | --- | --- |
| `GET /api/courses` | Always | Visible courses |
| `GET /api/my/courses` | Signed in | Enrolled courses, progress |
| `GET /api/my/skills` | Signed in | Level per skill (`?skill=` filter) |

The three calls run in parallel. A failure of a "signed in" call does not prevent the catalogue
from being shown.

Course fields (`courseResponse` in `course-service/internal/handlers/courses.go`): `slug`, `id`,
`title`, `description`, `category`, `difficulty`, `isPublic`, `moduleCount`, `labCount`,
`enrollmentCount`, `prerequisites[]`, `skills[]`, `badge {name, icon}`, `inPerson`, `sessions[]`,
`xpRequired`.

## Header and "Reprendre" hero

- A "Reprendre" block appears if the user has at least one enrolled course. It shows the first
  course **in progress** (progress strictly between 0 and 100 %), otherwise the first enrolled
  course.
- Content: thumbnail, the « Reprendre » label, title, progress bar, « X/Y modules · Z % », a
  « Continuer ▸ » button. The whole block links to `/courses/:slug`.
- Progress comes from `GET /api/my/courses` (`completedLabs` / `labCount`, see
  [090](090-gaps-and-defects.md) for what these names really count).

## Filters

All filters combine, and the list updates without a reload.

- **Search**: a text field, debounced by 280 ms. It matches title, description and category,
  case-insensitively.
- **Categories**: chips, shown only when there are at least two categories. A « Tout » chip resets
  the selection. Each chip has a colour dot and the number of courses. Several can be selected.
- **Difficulty**: a range slider with two thumbs, from « Débutant » to « Avancé ». A course passes
  if its difficulty is within the range; the full range (the default, shown « Toutes ») shows
  every course, those without a difficulty included.
- **Skill**: the `?skill=<slug>` URL parameter, set by skill links.
  - A course passes the filter if it teaches that skill and if its difficulty is at least the
    user's level for that skill (`beginner` < `intermediate` < `advanced`). Without a known level,
    every course of the skill passes.
  - A banner recalls the filter (« linux (Intermédiaire et +) ») with a button to remove it, which
    also removes the parameter from the URL.
- The « N cours » counter is shown above the grid.
- The category, difficulty and search filters **may** be reflected in the URL (improvement; today
  only `skill` is).

## Sorting and layout

- Courses sorted by title (`localeCompare` with `fr`, ignoring accents and case).
- Responsive grid of cards (`.cat-grid`, a decided change of [005](005-ui-theme-navigation.md)):
  2 columns on mobile, 3 from 768 px, 4 from 1024 px, 5 from 1280 px, with larger cards than the
  Astro page (140 px thumbnail, 104 px on mobile).
- Each time a filter changes (search, difficulty, category, skill), the grid remounts and its
  cards rise and fade in one after another (35 ms apart, capped at 15 cards; `opacity` and
  `transform` only). No animation under `prefers-reduced-motion: reduce`.
- In light mode, the page content sits on a white panel over the background drawing; dark mode
  has no drawing and no panel.
- Empty state: « Aucun cours ne correspond à votre recherche. »

## Course card

The whole card is a link to `/courses/:slug` (a real `<a>`, not an `onclick`).

- **Thumbnail**: the category icon on the category colour. The API returns no `thumbnail`
  ([090](090-gaps-and-defects.md)).
- **Category label**: icon and label.
- **Title**, two lines at most.
- **Badges**: difficulty (green / orange / red), « 🏢 Présentiel » if `inPerson`,
  « ⚡ N XP requis » if `xpRequired > 0`, « +200 XP », and up to three skills (« 🎯 linux », link
  to `/skills/:slug`).
- **Progress bar** if the user is enrolled.

### Known categories

| Slug | Label | Icon |
| --- | --- | --- |
| `linux` | Linux | 🐧 |
| `devops` | DevOps | ⚙️ |
| `kubernetes` | Kubernetes | ☸️ |
| `docker` | Docker | 🐋 |
| `networking` | Réseaux | 🌐 |
| `security` | Sécurité | 🛡️ |
| `programming` | Programmation | 💻 |
| `python` | Python | 🐍 |
| `git` | Git | 🔀 |
| `cloud` | Cloud | ☁️ |
| `database` | Base de données | 🗄️ |
| `internal-testing` | Tests internes | 🧪 |

An unknown category shows its slug with a capital letter and spaces instead of hyphens, and the
📖 icon.

### Thumbnail icon

When a course has no thumbnail, the icon is derived from the words of its category and skills, in
this order: pentest / hack / exploit / cyber / vuln / ctf → 🔐; security → 🛡️;
linux / unix / bash / shell → 🐧; git / gitlab / github / ci-cd / devops / pipeline → ⚙️;
docker / container / podman → 🐋; kubernetes / k8s / helm → ☸️; python → 🐍;
network / réseau / tcp / ip / wifi → 🌐; cloud / aws / azure / gcp → ☁️;
database / sql / postgres / mysql → 🗄️; web / http / html / css / frontend → 🌍;
javascript / typescript / node → 📦; go / golang → 🦫; otherwise the category icon.

### Category colours

Eight colours (blue, orange, green, violet, pink, cyan, red, indigo) are assigned in the order the
categories appear, so each category keeps the same colour on the page.

## Acceptance criteria

- Not signed in: the catalogue is shown, with no "Reprendre" block and no progress bar.
- `?skill=linux` shows only the courses that have `linux` in `skills`.
- Searching "kube" then choosing « Avancé » combines both filters.
- A course title containing HTML is shown as text.

## Sources

`frontend/src/pages/courses/index.astro`; `course-service/internal/handlers/courses.go`
(`ListCourses`, `courseResponse`).
