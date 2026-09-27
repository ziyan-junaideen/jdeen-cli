class JdeenCli < Formula
  desc "Command-line client for the JDeen JSON:API"
  homepage "https://github.com/ziyan-junaideen/jdeen-cli"
  url "ssh://git@github.com/ziyan-junaideen/jdeen-cli.git", tag: "v0.2.0"
  license "MIT"
  head "ssh://git@github.com/ziyan-junaideen/jdeen-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/ziyan-junaideen/jdeen-cli/internal/cli.version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags, output: bin/"jdeen"), "./cmd/jdeen"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/jdeen --version")
    assert_match "JDeen JSON:API", shell_output("#{bin}/jdeen --help")
  end
end
