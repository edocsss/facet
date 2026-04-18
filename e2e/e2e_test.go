//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func TestE2E_Docker(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found, skipping Docker E2E")
	}

	goarch := "amd64"
	if runtime.GOARCH == "arm64" {
		goarch = "arm64"
	}

	build := exec.Command("go", "build", "-o", "facet-linux", "..")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}
	t.Cleanup(func() { os.Remove("facet-linux") })

	dockerBuild := exec.Command("docker", "build", "-t", "facet-e2e", "-f", "Dockerfile.ubuntu", ".")
	if out, err := dockerBuild.CombinedOutput(); err != nil {
		t.Fatalf("docker build failed: %s\n%s", err, out)
	}

	dockerRun := exec.Command("docker", "run", "--rm",
		"-e", "FACET_E2E_REAL_PACKAGES=1",
		"facet-e2e")
	dockerRun.Stdout = os.Stdout
	dockerRun.Stderr = os.Stderr
	if err := dockerRun.Run(); err != nil {
		t.Fatalf("E2E tests failed: %s", err)
	}
}

func TestE2E_Native(t *testing.T) {
	harness := exec.Command("bash", "harness.sh")
	harness.Env = os.Environ()
	harness.Stdout = os.Stdout
	harness.Stderr = os.Stderr
	if err := harness.Run(); err != nil {
		t.Fatalf("Native E2E tests failed: %s", err)
	}
}
