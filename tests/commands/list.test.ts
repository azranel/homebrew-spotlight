import { describe, it, expect, beforeEach, afterEach, mock } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

function git(args: string[], cwd: string): void {
  Bun.spawnSync(["git", ...args], { cwd, stdout: "pipe", stderr: "pipe" });
}

describe("list command", () => {
  let mainRepo: string;
  let originalCwd: string;

  beforeEach(() => {
    originalCwd = process.cwd();
    mainRepo = mkdtempSync(path.join(tmpdir(), "spotlight-list-test-"));
    git(["init", "."], mainRepo);
    git(["commit", "--allow-empty", "-m", "init"], mainRepo);
  });

  afterEach(() => {
    process.chdir(originalCwd);
    // Clean up worktrees
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
        Bun.spawnSync(["git", "worktree", "remove", "--force", wtPath], { cwd: mainRepo });
      }
    }
    rmSync(mainRepo, { recursive: true, force: true });
  });

  it("lists worktrees with correct name, path, and branch", async () => {
    git(["branch", "feature-test"], mainRepo);
    const worktreePath = path.join(mainRepo, "..", "spotlight-wt-list");
    git(["worktree", "add", worktreePath, "feature-test"], mainRepo);

    process.chdir(mainRepo);

    const logs: string[] = [];
    const originalLog = console.log;
    console.log = (...args: unknown[]) => logs.push(args.join(" "));

    const { listCommand } = await import("../../src/commands/list");
    await listCommand();

    console.log = originalLog;

    const output = logs.join("\n");
    expect(output).toContain("NAME");
    expect(output).toContain("BRANCH");
    expect(output).toContain("(main)");
    expect(output).toContain("feature-test");

    rmSync(worktreePath, { recursive: true, force: true });
  });

  it("shows message when no linked worktrees exist", async () => {
    process.chdir(mainRepo);

    const logs: string[] = [];
    const originalLog = console.log;
    console.log = (...args: unknown[]) => logs.push(args.join(" "));

    const { listCommand } = await import("../../src/commands/list");
    await listCommand();

    console.log = originalLog;

    const output = logs.join("\n");
    expect(output).toContain("No linked worktrees found");
  });

  it("exits with error when not in a git repo", async () => {
    const nonGitDir = mkdtempSync(path.join(tmpdir(), "spotlight-nogit-"));
    process.chdir(nonGitDir);

    const errors: string[] = [];
    const originalError = console.error;
    console.error = (...args: unknown[]) => errors.push(args.join(" "));

    // Mock process.exit to throw so execution stops (like real exit would)
    const originalExit = process.exit;
    process.exit = ((code: number) => {
      throw new Error(`EXIT_${code}`);
    }) as unknown as typeof process.exit;

    const { listCommand } = await import("../../src/commands/list");

    let exitCode: number | null = null;
    try {
      await listCommand();
    } catch (e: unknown) {
      const msg = (e as Error).message;
      if (msg.startsWith("EXIT_")) {
        exitCode = parseInt(msg.slice(5));
      }
    }

    console.error = originalError;
    process.exit = originalExit;

    expect(exitCode).toBe(1);
    expect(errors.join("\n")).toContain("Not in a git repository");

    rmSync(nonGitDir, { recursive: true, force: true });
  });
});
