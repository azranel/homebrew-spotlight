import { describe, test, expect, beforeEach, afterEach, spyOn } from "bun:test";
import { mkdtempSync, rmSync, readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { acquireLock, releaseLock, type LockData } from "../../src/lib/lock";

const LOCK_FILENAME = ".spotlight-sync.lock";

describe("lock", () => {
  let tempDir: string;

  beforeEach(() => {
    tempDir = mkdtempSync(join(tmpdir(), "spotlight-lock-test-"));
  });

  afterEach(() => {
    rmSync(tempDir, { recursive: true, force: true });
  });

  describe("acquireLock", () => {
    test("creates lock file with correct JSON when none exists", () => {
      acquireLock(tempDir, "my-feature");

      const lockPath = join(tempDir, LOCK_FILENAME);
      expect(existsSync(lockPath)).toBe(true);

      const content = JSON.parse(readFileSync(lockPath, "utf-8")) as LockData;
      expect(content.pid).toBe(process.pid);
      expect(content.worktree).toBe("my-feature");
      expect(typeof content.startedAt).toBe("string");
      // Verify it's a valid ISO 8601 timestamp
      expect(Number.isNaN(new Date(content.startedAt).getTime())).toBe(false);
    });

    test("succeeds after cleaning up stale lock with dead PID", () => {
      // Write a stale lock with a PID that definitely doesn't exist
      const staleLock: LockData = {
        pid: 999999,
        worktree: "old-feature",
        startedAt: "2026-01-01T00:00:00.000Z",
      };
      const lockPath = join(tempDir, LOCK_FILENAME);
      require("node:fs").writeFileSync(
        lockPath,
        JSON.stringify(staleLock, null, 2),
      );

      const stderrSpy = spyOn(console, "error").mockImplementation(() => {});

      acquireLock(tempDir, "new-feature");

      // Verify warning was printed
      expect(stderrSpy).toHaveBeenCalledTimes(1);
      const warning = stderrSpy.mock.calls[0][0] as string;
      expect(warning).toContain("PID 999999");
      expect(warning).toContain('worktree "old-feature"');
      expect(warning).toContain("did not shut down cleanly");
      expect(warning).toContain("Cleaning up stale lock");

      stderrSpy.mockRestore();

      // Verify new lock was created
      const content = JSON.parse(readFileSync(lockPath, "utf-8")) as LockData;
      expect(content.pid).toBe(process.pid);
      expect(content.worktree).toBe("new-feature");
    });

    test("throws when active lock exists (current process PID)", () => {
      // Write a lock with the current process PID (definitely alive)
      const activeLock: LockData = {
        pid: process.pid,
        worktree: "active-feature",
        startedAt: "2026-04-15T10:00:00.000Z",
      };
      const lockPath = join(tempDir, LOCK_FILENAME);
      require("node:fs").writeFileSync(
        lockPath,
        JSON.stringify(activeLock, null, 2),
      );

      expect(() => acquireLock(tempDir, "another-feature")).toThrow(
        `Another sync is already running (PID ${process.pid}, worktree "active-feature", started 2026-04-15T10:00:00.000Z). Only one sync per repository is allowed.`,
      );
    });
  });

  describe("releaseLock", () => {
    test("removes existing lock file", () => {
      const lockPath = join(tempDir, LOCK_FILENAME);
      acquireLock(tempDir, "my-feature");
      expect(existsSync(lockPath)).toBe(true);

      releaseLock(tempDir);
      expect(existsSync(lockPath)).toBe(false);
    });

    test("does not throw when no lock file exists", () => {
      expect(() => releaseLock(tempDir)).not.toThrow();
    });
  });
});
