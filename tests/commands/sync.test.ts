import { describe, it, expect, beforeEach, afterEach } from "bun:test";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import {
  createCheckpoint,
  checkoutCommit,
  forceCheckoutRef,
  getCurrentRef,
  getHeadSha,
  getTreeSha,
  hasUncommittedChanges,
  amendCheckpoint,
} from "../../src/lib/git";

function git(args: string[], cwd: string): string {
  const result = Bun.spawnSync(["git", ...args], {
    cwd,
    stdout: "pipe",
    stderr: "pipe",
  });
  return result.stdout.toString().trim();
}

describe("sync command", () => {
  let mainRepo: string;
  let originalCwd: string;

  beforeEach(() => {
    originalCwd = process.cwd();
    mainRepo = mkdtempSync(path.join(tmpdir(), "spotlight-sync-test-"));
    git(["init", "."], mainRepo);
    git(["config", "user.email", "test@test.com"], mainRepo);
    git(["config", "user.name", "Test"], mainRepo);
    git(["commit", "--allow-empty", "-m", "init"], mainRepo);
  });

  afterEach(() => {
    process.chdir(originalCwd);
    // Clean up worktrees before removing repo
    const result = Bun.spawnSync(["git", "worktree", "list", "--porcelain"], {
      cwd: mainRepo,
      stdout: "pipe",
    });
    const output = result.stdout.toString();
    const worktreePaths = output
      .split("\n")
      .filter((l: string) => l.startsWith("worktree "))
      .map((l: string) => l.slice("worktree ".length));

    for (const wtPath of worktreePaths) {
      if (wtPath !== mainRepo) {
        Bun.spawnSync(["git", "worktree", "remove", "--force", wtPath], {
          cwd: mainRepo,
        });
      }
    }
    rmSync(mainRepo, { recursive: true, force: true });
  });

  it("initial sync creates checkpoint and checks it out", () => {
    // Create a worktree
    git(["branch", "feature-a"], mainRepo);
    const worktreePath = path.join(mainRepo, "..", "spotlight-wt-sync-1");
    git(["worktree", "add", worktreePath, "feature-a"], mainRepo);

    // Make a change in the worktree
    writeFileSync(path.join(worktreePath, "new-file.txt"), "hello from worktree");

    // Verify there are uncommitted changes
    expect(hasUncommittedChanges(worktreePath)).toBe(true);

    // Create checkpoint in worktree
    const checkpointSha = createCheckpoint(worktreePath);
    expect(checkpointSha).toMatch(/^[0-9a-f]{40}$/);

    // Checkout checkpoint in main repo
    const originalRef = getCurrentRef(mainRepo);
    checkoutCommit(checkpointSha, mainRepo);

    // Verify main repo now has the file from the worktree
    const headSha = getHeadSha(mainRepo);
    expect(headSha).toBe(checkpointSha);

    // Verify the file exists in main repo working tree
    const fileContent = Bun.spawnSync(["cat", path.join(mainRepo, "new-file.txt")], {
      stdout: "pipe",
    });
    expect(fileContent.stdout.toString().trim()).toBe("hello from worktree");

    // Restore for cleanup
    forceCheckoutRef(originalRef, mainRepo);

    rmSync(worktreePath, { recursive: true, force: true });
  });

  it("cleanup restores original branch", () => {
    const originalBranch = getCurrentRef(mainRepo);
    expect(originalBranch).toBe("main");

    // Create a second commit and checkout it (detached HEAD)
    git(["commit", "--allow-empty", "-m", "second"], mainRepo);
    const secondSha = getHeadSha(mainRepo);
    git(["checkout", "HEAD~1"], mainRepo);

    // Now we're on detached HEAD, force checkout original branch
    forceCheckoutRef(originalBranch, mainRepo);

    const currentRef = getCurrentRef(mainRepo);
    expect(currentRef).toBe("main");

    const headSha = getHeadSha(mainRepo);
    expect(headSha).toBe(secondSha);
  });

  it("sync fails when worktree not found", async () => {
    process.chdir(mainRepo);

    const errors: string[] = [];
    const originalError = console.error;
    console.error = (...args: unknown[]) => errors.push(args.join(" "));

    const originalExit = process.exit;
    process.exit = ((code: number) => {
      throw new Error(`EXIT_${code}`);
    }) as unknown as typeof process.exit;

    const { syncCommand } = await import("../../src/commands/sync");

    let exitCode: number | null = null;
    try {
      await syncCommand("nonexistent");
    } catch (e: unknown) {
      const msg = (e as Error).message;
      if (msg.startsWith("EXIT_")) {
        exitCode = parseInt(msg.slice(5));
      }
    }

    console.error = originalError;
    process.exit = originalExit;

    expect(exitCode).toBe(1);
    expect(errors.join("\n")).toContain("not found");
  });

  it("sync with no uncommitted changes uses worktree HEAD", () => {
    git(["branch", "feature-b"], mainRepo);
    const worktreePath = path.join(mainRepo, "..", "spotlight-wt-sync-2");
    git(["worktree", "add", worktreePath, "feature-b"], mainRepo);

    // No changes made — worktree is clean
    expect(hasUncommittedChanges(worktreePath)).toBe(false);

    // Should use the worktree's current HEAD
    const headSha = getHeadSha(worktreePath);
    expect(headSha).toMatch(/^[0-9a-f]{40}$/);

    rmSync(worktreePath, { recursive: true, force: true });
  });

  it("tree SHA comparison skips unnecessary checkout", () => {
    git(["branch", "feature-c"], mainRepo);
    const worktreePath = path.join(mainRepo, "..", "spotlight-wt-sync-3");
    git(["worktree", "add", worktreePath, "feature-c"], mainRepo);

    // Make a change and create initial checkpoint
    writeFileSync(path.join(worktreePath, "file.txt"), "content");
    const checkpointSha = createCheckpoint(worktreePath);
    const treeSha1 = getTreeSha(checkpointSha, worktreePath);

    // Amend with no real change (just re-stage the same files)
    const amendedSha = amendCheckpoint(worktreePath);
    const treeSha2 = getTreeSha(amendedSha, worktreePath);

    // Tree SHAs must match — no file content changed, so no checkout needed
    expect(treeSha2).toBe(treeSha1);

    rmSync(worktreePath, { recursive: true, force: true });
  });
});
