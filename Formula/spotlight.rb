# frozen_string_literal: true

# Homebrew formula for spotlight CLI
#
# Self-contained Go binary — no runtime dependencies.

class Spotlight < Formula
  desc "Sync git worktree changes to the main repository as checkpoints"
  homepage "https://github.com/azranel/spotlight"
  version "0.3.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/azranel/spotlight/releases/download/v0.3.0/spotlight-darwin-arm64.tar.gz"
    sha256 "97e07dfa6559b580ebc3902485f4af1c343639736113133eb011c4c87d07b7ed"
  elsif OS.mac? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.3.0/spotlight-darwin-amd64.tar.gz"
    sha256 "47cc1318f0b83e1d1a658531f15dc570f6a7f018f60df6d4799ca745444aeed4"
  elsif OS.linux? && Hardware::CPU.intel?
    url "https://github.com/azranel/spotlight/releases/download/v0.3.0/spotlight-linux-amd64.tar.gz"
    sha256 "7a28574522a5014572684644a208176c7f123a9280f30ca2bdd17b35837c35d0"
  end

  def install
    bin.install "spotlight"
  end

  test do
    assert_match "Sync git worktree changes", shell_output("#{bin}/spotlight --help")
  end
end
