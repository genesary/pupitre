// A tiny stand-in for the GitLab REST API read by the lab step checks
// (docs/specs/107-backend-labs-and-checker.md). Tests describe a learner's
// project with POST /__fake/projects; pupitre-api then reads it through
// /api/v4/… like a real GitLab (GITLAB_BASE_URL).
//
// Adapted from e2e/fake-gitlab.mjs of ebuildy/pupitre2: same server and same
// /__fake/projects call, answering the requests of the checker of
// genesary/pupitre (apps/api/internal/checker, GitLab's Go client).
//
// A project:
//   path            e-learning/<username>
//   defaultBranch   'main' by default
//   branches        { <name>: { commits: [message…], files: [path…] } }: the
//                   commits and changed files of the branch against main
//   mergeRequests   [{ title, source_branch, author, commits: [message…],
//                   pipeline: 'success' | 'failed' | … }], all open
import { createServer } from 'node:http';

const port = Number(process.env.PORT ?? 3103);
const byPath = new Map();
const byId = new Map();
let nextId = 1;

function send(res, status, body) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
}

const commit = (message, i) => ({ id: `c${i}`, short_id: `c${i}`, title: message.split('\n')[0], message });

createServer(async (req, res) => {
  const url = new URL(req.url, `http://localhost:${port}`);
  if (req.method === 'POST' && url.pathname === '/__fake/projects') {
    let raw = '';
    for await (const chunk of req) raw += chunk;
    const p = JSON.parse(raw);
    const existing = byPath.get(p.path);
    const project = { id: existing?.id ?? nextId++, defaultBranch: 'main', branches: {}, mergeRequests: [], ...p };
    byPath.set(p.path, project);
    byId.set(String(project.id), project);
    return send(res, 200, { id: project.id });
  }
  if (url.pathname === '/health') return send(res, 200, { ok: true });

  if (!url.pathname.startsWith('/api/v4/projects/')) return send(res, 404, { message: 'not found' });
  const parts = url.pathname.slice('/api/v4/projects/'.length).split('/');
  const key = decodeURIComponent(parts[0]);
  const project = byId.get(key) ?? byPath.get(key);
  if (!project) return send(res, 404, { message: '404 Project Not Found' });
  const rest = parts.slice(1).map(decodeURIComponent);

  if (rest.length === 0) {
    return send(res, 200, {
      id: project.id,
      name: project.path.split('/').at(-1),
      path_with_namespace: project.path,
      default_branch: project.defaultBranch,
    });
  }

  if (rest[0] === 'repository' && rest[1] === 'branches') {
    const names = [project.defaultBranch, ...Object.keys(project.branches)];
    return send(
      res,
      200,
      names.map((name) => {
        const commits = project.branches[name]?.commits ?? [];
        return { name, commit: commits.length > 0 ? commit(commits.at(-1), 0) : null };
      }),
    );
  }

  if (rest[0] === 'repository' && rest[1] === 'compare') {
    const branch = project.branches[url.searchParams.get('to')] ?? {};
    return send(res, 200, {
      commits: (branch.commits ?? []).map(commit),
      diffs: (branch.files ?? []).map((file) => ({ old_path: file, new_path: file })),
    });
  }

  if (rest[0] === 'merge_requests') {
    const mrs = project.mergeRequests.map((mr, i) => ({ iid: i + 1, state: 'opened', ...mr, author: { username: mr.author } }));
    if (rest.length === 1) {
      const state = url.searchParams.get('state');
      return send(
        res,
        200,
        mrs.filter((mr) => !state || mr.state === state).map(({ commits: _commits, pipeline: _pipeline, ...mr }) => mr),
      );
    }
    const mr = mrs.find((m) => String(m.iid) === rest[1]);
    if (!mr) return send(res, 404, { message: '404 Not found' });
    if (rest[2] === 'commits') return send(res, 200, (mr.commits ?? []).map(commit));
    const { commits: _commits, pipeline, ...detail } = mr;
    return send(res, 200, { ...detail, head_pipeline: pipeline ? { status: pipeline } : null });
  }

  send(res, 404, { message: 'not found' });
}).listen(port, () => console.log(`fake gitlab on ${port}`));
