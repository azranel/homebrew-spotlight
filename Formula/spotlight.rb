# frozen_string_literal: true

# Homebrew formula for spotlight CLI
#
# Self-contained Go binary — no runtime dependencies.

class Spotlight < Formula
  desc "Sync git worktree changes to the main repository as checkpoints"
  homepage "https://github.com/azranel/spotlight"
  version "0.2.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/azranel/spotlight/releases/download/v0.2.0/spotlight-darwin-arm64.tar.gz"
    sha256 "f5e67564bed0e31a47a4756c74f276f16680c817b0b79f3077bd9ee809eea965"
  elsif OS.mac? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.2.0/spotlight-darwin-amd64.tar.gz"
    sha256 "e01ca8647937a19c62317855f873c62e8be1efea8e40ab32324f7542a5182b0c"
  elsif OS.linux? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.2.0/spotlight-linux-amd64.tar.gz"
    sha256 "20b9303248e4c7e2ed547a4ff2b1dd5168c15ced2c4f01ecde30475b585788c2"
  end

  def install
    bin.install "spotlight"
  end

  test do
    assert_match "Sync git worktree changes", shell_output("#{bin}/spotlight --help")
  end
end
