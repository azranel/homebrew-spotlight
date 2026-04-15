#!/usr/bin/env bun
import { Command } from "commander";
import { listCommand } from "./commands/list";
import { syncCommand } from "./commands/sync";

const program = new Command();

program
  .name("spotlight")
  .description(
    "Sync git worktree changes to the main repository as checkpoints",
  )
  .version("0.1.0");

program
  .command("list")
  .description("List all git worktrees and their names")
  .action(listCommand);

program
  .command("sync <worktree>")
  .description("Sync a worktree's changes to the main repository")
  .action(syncCommand);

program.parse();
