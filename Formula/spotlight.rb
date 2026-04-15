# frozen_string_literal: true

# Homebrew formula for spotlight CLI
#
# To use this formula:
#   1. Create a GitHub release with the bundled dist/spotlight script
#   2. Update the url and sha256
#   3. Host this formula in a Homebrew tap repository
#   4. Install: brew tap <user>/spotlight && brew install spotlight

class Spotlight < Formula
  desc "Sync git worktree changes to the main repository as checkpoints"
  homepage "https://github.com/azranel/spotlight"
  version "0.1.0"
  license "MIT"

  url "https://github.com/azranel/spotlight/releases/download/v0.1.0/spotlight-v0.1.0.tar.gz"
  sha256 "412e951ca7b5ea8d830c7930f55f9775d1b844dae44141143936f2a94a4be26e"

  depends_on "oven-sh/bun/bun"

  def install
    bin.install "spotlight"
  end

  test do
    assert_match "Sync git worktree changes", shell_output("#{bin}/spotlight --help")
  end
end
