import { isGitRoot, listWorktrees } from "../lib/git";

export async function listCommand(): Promise<void> {
  if (!isGitRoot()) {
    console.error("Error: Not in a git repository root directory.");
    process.exit(1);
  }

  const worktrees = listWorktrees();
  const linked = worktrees.filter((wt) => !wt.bare);

  if (linked.length <= 1) {
    console.log("No linked worktrees found.");
    console.log(
      'Use "git worktree add <path> <branch>" to create a worktree.',
    );
    return;
  }

  const mainWorktree = linked[0];
  const nameWidth = Math.max(
    4,
    ...linked.slice(1).map((wt) => worktreeName(wt.path).length),
  );
  const branchWidth = Math.max(
    6,
    ...linked.map((wt) => (wt.branch ?? "detached").length),
  );

  console.log(
    `${"NAME".padEnd(nameWidth)}  ${"BRANCH".padEnd(branchWidth)}  PATH`,
  );

  for (const wt of linked) {
    const isMain = wt.path === mainWorktree.path;
    const name = isMain ? "(main)" : worktreeName(wt.path);
    const branch = wt.branch
      ? wt.branch.replace("refs/heads/", "")
      : "detached";

    console.log(
      `${name.padEnd(nameWidth)}  ${branch.padEnd(branchWidth)}  ${wt.path}`,
    );
  }
}

function worktreeName(worktreePath: string): string {
  return worktreePath.split("/").pop() ?? worktreePath;
}
