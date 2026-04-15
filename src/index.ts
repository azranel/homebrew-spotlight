#!/usr/bin/env bun
import { Command } from "commander";
import { listCommand } from "./commands/list";
import { syncCommand } from "./commands/sync";

const pkg = await Bun.file(new URL("../package.json", import.meta.url)).json();

const program = new Command();

program
  .name("spotlight")
  .description(pkg.description)
  .version(pkg.version);

program
  .command("list")
  .description("List all git worktrees and their names")
  .action(listCommand);

program
  .command("sync <worktree>")
  .description("Sync a worktree's changes to the main repository")
  .action(syncCommand);

program.parse();
