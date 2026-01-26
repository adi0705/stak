# Homebrew Formula for stak
# To use this formula:
# 1. Create a homebrew tap: brew tap adi0705/stak https://github.com/adi0705/homebrew-stak
# 2. Install: brew install adi0705/stak/stak

class Stak < Formula
  desc "CLI tool for managing stacked pull requests"
  homepage "https://github.com/adi0705/stak"
  version "1.0.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/adi0705/stak/releases/download/v1.0.0/stak-darwin-arm64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_FOR_ARM64"
    else
      url "https://github.com/adi0705/stak/releases/download/v1.0.0/stak-darwin-amd64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_FOR_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/adi0705/stak/releases/download/v1.0.0/stak-linux-arm64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_FOR_LINUX_ARM64"
    else
      url "https://github.com/adi0705/stak/releases/download/v1.0.0/stak-linux-amd64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_FOR_LINUX_AMD64"
    end
  end

  def install
    bin.install "stak-darwin-arm64" => "stak" if Hardware::CPU.arm? && OS.mac?
    bin.install "stak-darwin-amd64" => "stak" if Hardware::CPU.intel? && OS.mac?
    bin.install "stak-linux-arm64" => "stak" if Hardware::CPU.arm? && OS.linux?
    bin.install "stak-linux-amd64" => "stak" if Hardware::CPU.intel? && OS.linux?
  end

  test do
    system "#{bin}/stak", "--version"
  end
end
