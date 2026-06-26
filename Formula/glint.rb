class Glint < Formula
  desc "Local linter and validator for .gitlab-ci.yml pipelines"
  homepage "https://git.k3nny.fr/k3nny/glint"
  url "https://git.k3nny.fr/k3nny/glint/archive/v0.3.1.tar.gz"
  # Update sha256 on each release: sha256sum glint-vX.Y.Z.tar.gz
  sha256 ""
  license "Apache-2.0"
  head "https://git.k3nny.fr/k3nny/glint.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build",
           "-ldflags", "-X main.version=#{version}",
           "-o", bin/"glint",
           "./cmd/glint/..."
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/glint --version")
    (testpath/".gitlab-ci.yml").write <<~YAML
      stages: [build]
      build-job:
        stage: build
        script: echo ok
    YAML
    system bin/"glint", "check", ".gitlab-ci.yml"
  end
end
