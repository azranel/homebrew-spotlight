import { existsSync, readFileSync, unlinkSync } from "node:fs";
import { join } from "node:path";

const LOCK_FILENAME = ".spotlight-sync.lock";

export interface LockData {
  pid: number;
  worktree: string;
  startedAt: string;
}

function lockPath(repoRoot: string): string {
  return join(repoRoot, LOCK_FILENAME);
}

function isPidAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

export function acquireLock(repoRoot: string, worktreeName: string): void {
  const path = lockPath(repoRoot);

  if (existsSync(path)) {
    const content = readFileSync(path, "utf-8");
    const lockData = JSON.parse(content) as LockData;

    if (isPidAlive(lockData.pid)) {
      throw new Error(
        `Another sync is already running (PID ${lockData.pid}, worktree "${lockData.worktree}", started ${lockData.startedAt}). Only one sync per repository is allowed.`,
      );
    }

    console.error(
      `Warning: Previous sync (PID ${lockData.pid}, worktree "${lockData.worktree}") did not shut down cleanly. Your repository may be in a dirty state. Cleaning up stale lock.`,
    );
    unlinkSync(path);
  }

  const data: LockData = {
    pid: process.pid,
    worktree: worktreeName,
    startedAt: new Date().toISOString(),
  };

  Bun.write(path, JSON.stringify(data, null, 2));
}

export function releaseLock(repoRoot: string): void {
  const path = lockPath(repoRoot);
  if (existsSync(path)) {
    unlinkSync(path);
  }
}

export function registerCleanupHandlers(
  cleanup: () => Promise<void>,
): void {
  let cleaning = false;

  const handler = async () => {
    if (cleaning) return;
    cleaning = true;
    await cleanup();
    process.exit(0);
  };

  process.on("SIGINT", handler);
  process.on("SIGTERM", handler);
}
