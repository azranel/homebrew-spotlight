export interface Worktree {
  path: string;
  head: string;
  branch: string | null;
  bare: boolean;
}

function runGit(args: string[], cwd?: string): { stdout: string; stderr: string } {
  const result = Bun.spawnSync(["git", ...args], {
    cwd,
    stdout: "pipe",
    stderr: "pipe",
  });

  const stdout = result.stdout.toString().trim();
  const stderr = result.stderr.toString().trim();

  if (result.exitCode !== 0) {
    throw new Error(
      `git ${args.join(" ")} failed (exit ${result.exitCode}): ${stderr}`,
    );
  }

  return { stdout, stderr };
}

export function listWorktrees(cwd?: string): Worktree[] {
  const { stdout } = runGit(["worktree", "list", "--porcelain"], cwd);
  if (!stdout) return [];

  const blocks = stdout.split("\n\n");
  const worktrees: Worktree[] = [];

  for (const block of blocks) {
    const lines = block.trim().split("\n");
    if (lines.length === 0 || !lines[0]) continue;

    let path = "";
    let head = "";
    let branch: string | null = null;
    let bare = false;

    for (const line of lines) {
      if (line.startsWith("worktree ")) {
        path = line.slice("worktree ".length);
      } else if (line.startsWith("HEAD ")) {
        head = line.slice("HEAD ".length);
      } else if (line.startsWith("branch ")) {
        branch = line.slice("branch ".length);
      } else if (line === "detached") {
        branch = null;
      } else if (line === "bare") {
        bare = true;
      }
    }

    if (path) {
      worktrees.push({ path, head, branch, bare });
    }
  }

  return worktrees;
}

export function isGitRoot(cwd?: string): boolean {
  const { realpathSync } = require("node:fs");
  const dir = realpathSync(cwd ?? process.cwd());
  try {
    const { stdout } = runGit(["rev-parse", "--show-toplevel"], dir);
    return realpathSync(stdout) === dir;
  } catch {
    return false;
  }
}

export function getCurrentRef(cwd?: string): string {
  try {
    const { stdout } = runGit(["symbolic-ref", "--short", "HEAD"], cwd);
    return stdout;
  } catch {
    const { stdout } = runGit(["rev-parse", "HEAD"], cwd);
    return stdout;
  }
}

export function getHeadSha(cwd?: string): string {
  const { stdout } = runGit(["rev-parse", "HEAD"], cwd);
  return stdout;
}

export function hasUncommittedChanges(cwd?: string): boolean {
  const { stdout } = runGit(["status", "--porcelain"], cwd);
  return stdout.length > 0;
}

export function stashChanges(cwd?: string): boolean {
  const { stdout } = runGit(["stash", "-u"], cwd);
  return !stdout.includes("No local changes to save");
}

export function popStash(cwd?: string): { success: boolean; conflicted: boolean } {
  const result = Bun.spawnSync(["git", "stash", "pop"], {
    cwd,
    stdout: "pipe",
    stderr: "pipe",
  });

  const stderr = result.stderr.toString();

  if (result.exitCode === 0) {
    return { success: true, conflicted: false };
  }

  const conflicted = stderr.includes("CONFLICT");
  return { success: false, conflicted };
}

export function createCheckpoint(cwd: string): string {
  runGit(["add", "-A"], cwd);
  runGit(["commit", "-m", "spotlight checkpoint"], cwd);
  const { stdout } = runGit(["rev-parse", "HEAD"], cwd);
  return stdout;
}

export function amendCheckpoint(cwd: string): string {
  runGit(["add", "-A"], cwd);
  runGit(["commit", "--amend", "--no-edit"], cwd);
  const { stdout } = runGit(["rev-parse", "HEAD"], cwd);
  return stdout;
}

export function getTreeSha(commitSha: string, cwd?: string): string {
  const { stdout } = runGit(["rev-parse", `${commitSha}^{tree}`], cwd);
  return stdout;
}

export function checkoutCommit(sha: string, cwd?: string): void {
  runGit(["checkout", sha], cwd);
}

export function forceCheckoutRef(ref: string, cwd?: string): void {
  runGit(["checkout", "--force", ref], cwd);
}

export function softReset(sha: string, cwd: string): void {
  runGit(["reset", "--soft", sha], cwd);
}
