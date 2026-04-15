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
    sha256 "8625fb58efd618209808d69edb6cc1710fc9a7205e0abf35db04ac2f8527d135"
  elsif OS.mac? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.1/spotlight-darwin-x64.tar.gz"
    sha256 "d72c1360f11d57dedc2eec00149dbfc5b28914960506827c48591d97cb617bae"
  elsif OS.linux? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.1/spotlight-linux-x64.tar.gz"
    sha256 "ee62a5583e872007420fa805378002844f7c0964cc7bce1f964425a9c97add31"
  end

  def install
    bin.install "spotlight"
  end

  test do
    assert_match "Sync git worktree changes", shell_output("#{bin}/spotlight --help")
  end
end
