import { describe, test, expect, beforeAll, afterAll } from "bun:test";
import { mkdtempSync, writeFileSync, rmSync, realpathSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import {
  listWorktrees,
  isGitRoot,
  getCurrentRef,
  getHeadSha,
  hasUncommittedChanges,
  stashChanges,
  createCheckpoint,
  amendCheckpoint,
  getTreeSha,
  checkoutCommit,
  forceCheckoutRef,
  softReset,
} from "../../src/lib/git";

function git(args: string[], cwd: string): string {
  const result = Bun.spawnSync(["git", ...args], {
    cwd,
    stdout: "pipe",
    stderr: "pipe",
  });
  if (result.exitCode !== 0) {
    throw new Error(
      `git ${args.join(" ")} failed: ${result.stderr.toString()}`,
    );
  }
  return result.stdout.toString().trim();
}

function initRepo(dir: string): void {
  git(["init"], dir);
  git(["config", "user.email", "test@test.com"], dir);
  git(["config", "user.name", "Test"], dir);
  writeFileSync(path.join(dir, "file.txt"), "initial");
  git(["add", "-A"], dir);
  git(["commit", "-m", "initial commit"], dir);
}

describe("git operations", () => {
  let repoDir: string;
  let worktreeDir: string;

  beforeAll(() => {
    repoDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-")));
    initRepo(repoDir);

    // Create a branch for worktree
    git(["branch", "feature-branch"], repoDir);

    // Create a linked worktree
    worktreeDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-wt-")));
    // Remove the temp dir since git worktree add wants to create it
    rmSync(worktreeDir, { recursive: true });
    git(["worktree", "add", worktreeDir, "feature-branch"], repoDir);
  });

  afterAll(() => {
    // Remove worktree first, then repo
    try {
      git(["worktree", "remove", "--force", worktreeDir], repoDir);
    } catch {
      // ignore
    }
    rmSync(repoDir, { recursive: true, force: true });
    rmSync(worktreeDir, { recursive: true, force: true });
  });

  describe("listWorktrees", () => {
    test("parses output correctly with multiple worktrees", () => {
      const worktrees = listWorktrees(repoDir);
      expect(worktrees.length).toBeGreaterThanOrEqual(2);

      const main = worktrees.find((w) => w.path === repoDir);
      expect(main).toBeDefined();
      expect(main!.head).toMatch(/^[0-9a-f]{40}$/);
      expect(main!.branch).toContain("refs/heads/");
      expect(main!.bare).toBe(false);

      const linked = worktrees.find((w) => w.path === worktreeDir);
      expect(linked).toBeDefined();
      expect(linked!.branch).toBe("refs/heads/feature-branch");
    });

    test("returns only main when no linked worktrees", () => {
      const soloDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-solo-")));
      initRepo(soloDir);
      try {
        const worktrees = listWorktrees(soloDir);
        expect(worktrees.length).toBe(1);
        expect(worktrees[0].path).toBe(soloDir);
      } finally {
        rmSync(soloDir, { recursive: true, force: true });
      }
    });
  });

  describe("isGitRoot", () => {
    test("returns true for git repo root", () => {
      expect(isGitRoot(repoDir)).toBe(true);
    });

    test("returns false in non-git directory", () => {
      const nonGitDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-nogit-")));
      try {
        expect(isGitRoot(nonGitDir)).toBe(false);
      } finally {
        rmSync(nonGitDir, { recursive: true, force: true });
      }
    });
  });

  describe("getCurrentRef", () => {
    test("returns branch name when on a branch", () => {
      const ref = getCurrentRef(repoDir);
      // Should be a branch name like "main" or "master"
      expect(ref).toMatch(/^[a-zA-Z]/);
      expect(ref).not.toMatch(/^[0-9a-f]{40}$/);
    });

    test("returns SHA when in detached HEAD", () => {
      const detachedDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-detached-")));
      initRepo(detachedDir);
      const sha = git(["rev-parse", "HEAD"], detachedDir);
      git(["checkout", "--detach"], detachedDir);
      try {
        const ref = getCurrentRef(detachedDir);
        expect(ref).toBe(sha);
      } finally {
        rmSync(detachedDir, { recursive: true, force: true });
      }
    });
  });

  describe("getHeadSha", () => {
    test("returns 40-char hex SHA", () => {
      const sha = getHeadSha(repoDir);
      expect(sha).toMatch(/^[0-9a-f]{40}$/);
    });
  });

  describe("hasUncommittedChanges", () => {
    test("returns false when clean", () => {
      expect(hasUncommittedChanges(repoDir)).toBe(false);
    });

    test("returns true when dirty", () => {
      const dirtyDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-dirty-")));
      initRepo(dirtyDir);
      writeFileSync(path.join(dirtyDir, "new.txt"), "dirty");
      try {
        expect(hasUncommittedChanges(dirtyDir)).toBe(true);
      } finally {
        rmSync(dirtyDir, { recursive: true, force: true });
      }
    });
  });

  describe("stashChanges", () => {
    test("stashes untracked files with -u flag", () => {
      const stashDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-stash-")));
      initRepo(stashDir);
      const untrackedFile = path.join(stashDir, "untracked.txt");
      writeFileSync(untrackedFile, "untracked content");
      try {
        const stashed = stashChanges(stashDir);
        expect(stashed).toBe(true);

        // Verify untracked file is gone after stash
        const { existsSync } = require("node:fs");
        expect(existsSync(untrackedFile)).toBe(false);
      } finally {
        rmSync(stashDir, { recursive: true, force: true });
      }
    });

    test("returns false when nothing to stash", () => {
      const cleanDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-clean-")));
      initRepo(cleanDir);
      try {
        const stashed = stashChanges(cleanDir);
        expect(stashed).toBe(false);
      } finally {
        rmSync(cleanDir, { recursive: true, force: true });
      }
    });
  });

  describe("createCheckpoint", () => {
    test("returns new commit SHA", () => {
      const cpDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-cp-")));
      initRepo(cpDir);
      writeFileSync(path.join(cpDir, "checkpoint.txt"), "checkpoint");
      try {
        const sha = createCheckpoint(cpDir);
        expect(sha).toMatch(/^[0-9a-f]{40}$/);

        // Verify commit message
        const msg = git(["log", "-1", "--format=%s"], cpDir);
        expect(msg).toBe("spotlight checkpoint");
      } finally {
        rmSync(cpDir, { recursive: true, force: true });
      }
    });
  });

  describe("amendCheckpoint", () => {
    test("amends existing commit and returns new SHA", () => {
      const amendDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-amend-")));
      initRepo(amendDir);
      writeFileSync(path.join(amendDir, "first.txt"), "first");
      createCheckpoint(amendDir);
      const sha1 = getHeadSha(amendDir);

      writeFileSync(path.join(amendDir, "second.txt"), "second");
      try {
        const sha2 = amendCheckpoint(amendDir);
        expect(sha2).toMatch(/^[0-9a-f]{40}$/);
        expect(sha2).not.toBe(sha1);

        // Should still be "spotlight checkpoint" message
        const msg = git(["log", "-1", "--format=%s"], amendDir);
        expect(msg).toBe("spotlight checkpoint");
      } finally {
        rmSync(amendDir, { recursive: true, force: true });
      }
    });
  });

  describe("getTreeSha", () => {
    test("returns tree hash for a commit", () => {
      const sha = getHeadSha(repoDir);
      const treeSha = getTreeSha(sha, repoDir);
      expect(treeSha).toMatch(/^[0-9a-f]{40}$/);
      expect(treeSha).not.toBe(sha);
    });
  });

  describe("checkoutCommit", () => {
    test("checks out a specific commit (detached HEAD)", () => {
      const coDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-co-")));
      initRepo(coDir);
      const sha = getHeadSha(coDir);
      writeFileSync(path.join(coDir, "extra.txt"), "extra");
      git(["add", "-A"], coDir);
      git(["commit", "-m", "second"], coDir);

      try {
        checkoutCommit(sha, coDir);
        const currentSha = getHeadSha(coDir);
        expect(currentSha).toBe(sha);
      } finally {
        rmSync(coDir, { recursive: true, force: true });
      }
    });
  });

  describe("forceCheckoutRef", () => {
    test("succeeds even when working tree is dirty", () => {
      const forceDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-force-")));
      initRepo(forceDir);
      const branchName = git(["symbolic-ref", "--short", "HEAD"], forceDir);

      // Create second commit
      writeFileSync(path.join(forceDir, "second.txt"), "second");
      git(["add", "-A"], forceDir);
      git(["commit", "-m", "second"], forceDir);

      // Make working tree dirty
      writeFileSync(path.join(forceDir, "file.txt"), "dirty changes");

      try {
        // Should not throw despite dirty working tree
        forceCheckoutRef(branchName, forceDir);
        const ref = getCurrentRef(forceDir);
        expect(ref).toBe(branchName);
      } finally {
        rmSync(forceDir, { recursive: true, force: true });
      }
    });
  });

  describe("softReset", () => {
    test("resets to given SHA while preserving working directory", () => {
      const resetDir = realpathSync(mkdtempSync(path.join(tmpdir(), "spotlight-test-reset-")));
      initRepo(resetDir);
      const originalSha = getHeadSha(resetDir);

      writeFileSync(path.join(resetDir, "new.txt"), "new content");
      createCheckpoint(resetDir);

      try {
        softReset(originalSha, resetDir);
        const currentSha = getHeadSha(resetDir);
        expect(currentSha).toBe(originalSha);

        // Working directory should still have the file
        const { existsSync } = require("node:fs");
        expect(existsSync(path.join(resetDir, "new.txt"))).toBe(true);
      } finally {
        rmSync(resetDir, { recursive: true, force: true });
      }
    });
  });
});
