# frozen_string_literal: true

# Homebrew formula for spotlight CLI
#
# The release tarball contains a self-contained binary produced by
# `bun build --compile` — Bun is embedded, so no runtime dependency is needed.
#
# To use this formula:
#   1. Build release binaries in CI with `bun build --compile`
#   2. Create a GitHub release with platform-specific tarballs
#   3. Update the url and sha256 for each platform below
#   4. Host this formula in a Homebrew tap repository
#   5. Install: brew tap azranel/spotlight && brew install spotlight

class Spotlight < Formula
  desc "Sync git worktree changes to the main repository as checkpoints"
  homepage "https://github.com/azranel/spotlight"
  version "0.1.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.0/spotlight-darwin-arm64.tar.gz"
    sha256 "PLACEHOLDER_SHA256_DARWIN_ARM64"
  elsif OS.mac? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.0/spotlight-darwin-x64.tar.gz"
    sha256 "PLACEHOLDER_SHA256_DARWIN_X64"
  elsif OS.linux? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.0/spotlight-linux-x64.tar.gz"
    sha256 "PLACEHOLDER_SHA256_LINUX_X64"
  end

  def install
    bin.install "spotlight"
  end

  test do
    assert_match "Sync git worktree changes", shell_output("#{bin}/spotlight --help")
  end
end
