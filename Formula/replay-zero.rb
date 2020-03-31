class ReplayZero < Formula
  desc "Tool to turn local HTTP traffic into functional tests"
  homepage "https://github.com/intuit/ReplayZero"

  url "https://github.com/intuit/ReplayZero/releases/download/v0.0.2/replay-zero-osx.zip"
  version "0.0.2"
  sha256 "351f09ac1638aeb402d1198c3bc8862cc4df4082a6e662fda30c86f88d459a4d"

  head "https://github.com/intuit/ReplayZero.git"
  depends_on :arch => :intel

  def install
      bin.install "replay-zero-osx" => "replay-zero"
  end

  test do
    "replay-zero -V"
  end
end
