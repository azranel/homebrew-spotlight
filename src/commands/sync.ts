import { watch } from "chokidar";
import {
  isGitRoot,
  listWorktrees,
  getCurrentRef,
  getHeadSha,
  stashChanges,
  hasUncommittedChanges,
  createCheckpoint,
  amendCheckpoint,
  getTreeSha,
  checkoutCommit,
  forceCheckoutRef,
  popStash,
  softReset,
} from "../lib/git";
import { acquireLock, releaseLock, registerCleanupHandlers } from "../lib/lock";

const GREEN = "\x1b[32m";
const YELLOW = "\x1b[33m";
const RED = "\x1b[31m";
const RESET = "\x1b[0m";
const BOLD = "\x1b[1m";

export async function syncCommand(worktreeName: string): Promise<void> {
  const cwd = process.cwd();

  // --- STARTUP ---

  if (!isGitRoot()) {
    console.error(`${RED}Error:${RESET} Not in a git repository root directory.`);
    process.exit(1);
  }

  const worktrees = listWorktrees();
  const worktree = worktrees.find(
    (wt) => wt.path.split("/").pop() === worktreeName,
  );

  if (!worktree) {
    console.error(
      `${RED}Error:${RESET} Worktree "${worktreeName}" not found.`,
    );
    const linked = worktrees.filter((wt) => !wt.bare).slice(1);
    if (linked.length > 0) {
      console.error("Available worktrees:");
      for (const wt of linked) {
        console.error(`  - ${wt.path.split("/").pop()}`);
      }
    } else {
      console.error("No linked worktrees available.");
    }
    process.exit(1);
  }

  const worktreePath = worktree.path;

  acquireLock(cwd, worktreeName);

  const originalRef = getCurrentRef();
  const worktreeOriginalHead = getHeadSha(worktreePath);
  const didStash = stashChanges();

  // --- INITIAL SYNC ---

  let currentCheckoutSha: string;
  let hasCheckpoint: boolean;

  if (hasUncommittedChanges(worktreePath)) {
    currentCheckoutSha = createCheckpoint(worktreePath);
    hasCheckpoint = true;
  } else {
    currentCheckoutSha = getHeadSha(worktreePath);
    hasCheckpoint = false;
  }

  checkoutCommit(currentCheckoutSha);
  console.log(`${GREEN}\u2713${RESET} Synced ${worktreeName} \u2192 main repo`);

  // --- WATCH LOOP ---

  const watcher = watch(worktreePath, {
    ignored: [/(^|[/\\])\.git/, /node_modules/],
    ignoreInitial: true,
  });

  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  const onFileChange = () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(async () => {
      try {
        console.log(`${YELLOW}\u27F3${RESET} Change detected, syncing...`);

        let newSha: string;
        if (hasCheckpoint) {
          newSha = amendCheckpoint(worktreePath);
        } else {
          newSha = createCheckpoint(worktreePath);
          hasCheckpoint = true;
        }

        const newTreeSha = getTreeSha(newSha);
        const currentTreeSha = getTreeSha(currentCheckoutSha);

        if (newTreeSha !== currentTreeSha) {
          checkoutCommit(newSha);
          currentCheckoutSha = newSha;
          const now = new Date().toLocaleTimeString();
          console.log(
            `${GREEN}\u2713${RESET} Synced ${worktreeName} \u2192 main repo [${now}]`,
          );
        }
      } catch (error) {
        console.error(
          `${RED}Error during sync:${RESET} ${(error as Error).message}`,
        );
      }
    }, 300);
  };

  watcher.on("add", onFileChange);
  watcher.on("change", onFileChange);
  watcher.on("unlink", onFileChange);

  console.log(
    `${BOLD}Watching for changes...${RESET} (Ctrl+C to stop)`,
  );

  // --- CLEANUP ---

  let cleaningUp = false;

  const cleanup = async () => {
    if (cleaningUp) return;
    cleaningUp = true;

    if (debounceTimer) clearTimeout(debounceTimer);
    await watcher.close();

    forceCheckoutRef(originalRef);

    if (didStash) {
      const result = popStash();
      if (!result.success && result.conflicted) {
        console.error(
          `${YELLOW}Warning:${RESET} Stash pop had conflicts. Run 'git stash pop' manually to resolve.`,
        );
      }
    }

    if (hasCheckpoint) {
      softReset(worktreeOriginalHead, worktreePath);
    }

    releaseLock(cwd);

    console.log(`${GREEN}\u2713${RESET} Restored to ${originalRef}`);
  };

  registerCleanupHandlers(cleanup);

  // Keep the process alive
  await new Promise(() => {});
}
