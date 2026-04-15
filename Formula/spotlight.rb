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
  version "0.1.1"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.1/spotlight-darwin-arm64.tar.gz"
    sha256 "49e14acb2644e1665b26d2148c196d89a94a625356ce54ab460eb83a6d64eb29"
  elsif OS.mac? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.1/spotlight-darwin-x64.tar.gz"
    sha256 "3b765261d4d7cbbc3b71260e16b61d1ddbfad4c8d59ca6affb996d08d8c8d98b"
  elsif OS.linux? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.1/spotlight-linux-x64.tar.gz"
    sha256 "a7bbe45a798154c8635d4915a3e11592a48b2bbf2601a4dead58d8e2b7f15adb"
  end

  def install
    bin.install "spotlight"
  end

  test do
    assert_match "Sync git worktree changes", shell_output("#{bin}/spotlight --help")
  end
end
