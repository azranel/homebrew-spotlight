# frozen_string_literal: true

# Homebrew formula for spotlight CLI
#
# Self-contained Go binary — no runtime dependencies.

class Spotlight < Formula
  desc "Sync git worktree changes to the main repository as checkpoints"
  homepage "https://github.com/azranel/spotlight"
  version "0.1.1"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.1/spotlight-darwin-arm64.tar.gz"
    sha256 "PLACEHOLDER_SHA256_DARWIN_ARM64"
  elsif OS.mac? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.1/spotlight-darwin-amd64.tar.gz"
    sha256 "PLACEHOLDER_SHA256_DARWIN_AMD64"
  elsif OS.linux? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.1.1/spotlight-linux-amd64.tar.gz"
    sha256 "PLACEHOLDER_SHA256_LINUX_AMD64"
  end

  def install
    bin.install "spotlight"
  end

  test do
    assert_match "Sync git worktree changes", shell_output("#{bin}/spotlight --help")
  end
end
