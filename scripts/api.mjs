// Runs the Go commands of apps/api from the repository root, so the frontend
// scripts and the backend ones live side by side in package.json
// (docs/specs/108-backend-operations-and-testing.md). apps/api has no
// package.json: a frontend contributor needs no Go toolchain.
//
//   node scripts/api.mjs dev | test | lint | build | up | down
import { spawn } from 'node:child_process';
import { copyFileSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const API = fileURLToPath(new URL('../apps/api/', import.meta.url));

// `dev` builds the binary and runs it, rather than `go run`: on Windows the
// program `go run` starts outlives it when the parent is stopped.
const BINARY = process.platform === 'win32' ? 'bin/pupitre-api.exe' : 'bin/pupitre-api';

const COMMANDS = {
  dev: [
    ['go', 'build', '-o', BINARY, './cmd/pupitre-api'],
    [`${API}${BINARY}`],
  ],
  test: [['go', 'test', '-race', './...']],
  lint: [['golangci-lint', 'run', './...']],
  build: [
    ['go', 'build', './...'],
    ['go', 'vet', './...'],
  ],
  up: [['docker', 'compose', 'up', '-d', '--wait']],
  down: [['docker', 'compose', 'down']],
};

const name = process.argv[2];
const steps = COMMANDS[name];
if (!steps) {
  console.error(`usage: node scripts/api.mjs ${Object.keys(COMMANDS).join('|')}`);
  process.exit(2);
}

// The development settings live in apps/api/.env, created from the example.
if (name === 'dev' && !existsSync(`${API}.env`)) {
  copyFileSync(`${API}.env.example`, `${API}.env`);
  console.log('apps/api/.env created from .env.example');
}

// Stopping this script stops the command it runs.
let current;
for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => {
    current?.kill(signal);
    process.exit(130);
  });
}

for (const [command, ...args] of steps) {
  const code = await new Promise((resolve) => {
    const child = spawn(command, args, { cwd: API, stdio: 'inherit' });
    current = child;
    child.on('error', (error) => {
      console.error(`${command}: ${error.message}`);
      resolve(1);
    });
    child.on('exit', (status) => resolve(status ?? 1));
  });
  if (code !== 0) process.exit(code);
}
